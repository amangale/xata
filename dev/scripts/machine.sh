#!/bin/bash

set -euo pipefail

mode="${1:-create}"
instance_type="${INSTANCE_TYPE:-t4g.xlarge}"
owner="$(tailscale whois --json "$(tailscale ip | head -n1)" | jq -r .UserProfile.LoginName)"
region="${REGION:-eu-central-1}"

case "$mode" in
    create | start | stop | destroy) ;;
    *)
        echo "Usage: $0 [create|start|stop|destroy]" >&2
        exit 1
        ;;
esac

wait_for_ssh() {
    until tailscale ping --timeout 1s -c 1 "$machine_name" >/dev/null 2>&1; do
        sleep 1
    done
    until tailscale ssh "ubuntu@$machine_name" true >/dev/null 2>&1; do
        sleep 1
    done
}

# Return existing instance if it exists so we do not spam the sandbox with EC2
# instances. One per owner should be enough.
existing_machine="$(aws ec2 describe-instances --region "$region" \
    --filters "Name=tag:Owner,Values=$owner" "Name=instance-state-name,Values=pending,running,stopping,stopped" \
    --query "Reservations[].Instances[].[InstanceId,State.Name,Tags[?Key=='Name'].Value|[0],SecurityGroups[0].GroupId] | [0]" \
    --output text)"
if [[ "$existing_machine" != "None" ]]; then
    read -r existing_instance existing_state machine_name security_group <<<"$existing_machine"
    if [[ "$mode" == "stop" ]]; then
        aws ec2 stop-instances \
            --region "$region" \
            --instance-ids "$existing_instance" \
            --query 'StoppingInstances[0].CurrentState.Name' \
            --output text >&2
        aws ec2 wait instance-stopped --region "$region" --instance-ids "$existing_instance"
        exit 0
    fi
    if [[ "$mode" == "start" ]]; then
        if [[ "$existing_state" != "running" ]]; then
            aws ec2 start-instances \
                --region "$region" \
                --instance-ids "$existing_instance" \
                --query 'StartingInstances[0].CurrentState.Name' \
                --output text >&2
            aws ec2 wait instance-running --region "$region" --instance-ids "$existing_instance"
        fi
        wait_for_ssh
        echo "ubuntu@$machine_name"
        exit 0
    fi
    if [[ "$mode" == "destroy" ]]; then
        aws ec2 terminate-instances \
            --region "$region" \
            --instance-ids "$existing_instance" \
            --query 'TerminatingInstances[0].CurrentState.Name' \
            --output text >&2
        aws ec2 wait instance-terminated --region "$region" --instance-ids "$existing_instance"
        aws ec2 delete-security-group --region "$region" --group-id "$security_group"
        exit 0
    fi
    echo "Machine for $owner already exists" >&2
    echo "ubuntu@$machine_name"
    exit 0
fi
if [[ "$mode" != "create" ]]; then
    echo "No machine for $owner" >&2
    exit 1
fi

# Generate random memorable machine name using this little dict
dict=(
    anvil atlas banjo beacon boulder comet copper disco echo ember
    falcon fizzy galaxy harbor igloo lantern maple meteor nacho nebula
    orbit pickle pixel plasma rocket saffron tango thunder velvet waffle
)
machine_name="${MACHINE_NAME:-${dict[RANDOM % ${#dict[@]}]}-${dict[RANDOM % ${#dict[@]}]}}"

# Run machines on Ubuntu since that is the OS we use in prod as well. Also
# create a dedicated security group so they are behind a firewall by default.
# You have to use tailscale to access it.
ami="$(aws ec2 describe-images \
    --region "$region" \
    --owners 099720109477 \
    --filters Name=name,Values="ubuntu-minimal/images/hvm-ssd-gp3/ubuntu-noble-24.04-arm64-minimal-*" \
    --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
    --output text)"
vpc="$(aws ec2 describe-vpcs \
    --region "$region" \
    --filters Name=is-default,Values=true \
    --query 'Vpcs[0].VpcId' \
    --output text)"
security_group="$(aws ec2 create-security-group \
    --region "$region" \
    --group-name "$machine_name" \
    --description "$machine_name" \
    --vpc-id "$vpc" \
    --tag-specifications "ResourceType=security-group,Tags=[{Key=Name,Value=$machine_name},{Key=Owner,Value=$owner}]" \
    --query GroupId \
    --output text)"

# Launch machine and bootstrap tailscale for access
user_data="$(printf '#!/bin/bash\ncurl -fsSL https://tailscale.com/install.sh | sh\ntailscale up --ssh --hostname %q\n' "$machine_name")"
instance="$(aws ec2 run-instances \
    --region "$region" \
    --image-id "$ami" \
    --instance-type "$instance_type" \
    --security-group-ids "$security_group" \
    --metadata-options HttpTokens=required \
    --block-device-mappings 'DeviceName=/dev/sda1,Ebs={VolumeSize=100,VolumeType=gp3,Encrypted=true,DeleteOnTermination=true}' \
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=$machine_name},{Key=Owner,Value=$owner}]" \
    --user-data "$user_data" \
    --query 'Instances[0].InstanceId' \
    --output text)"

echo "Launched $instance as $machine_name" >&2

while true; do
    output="$(aws ec2 get-console-output \
        --region "$region" \
        --instance-id "$instance" \
        --latest \
        --query Output \
        --output text)"
    login_url="$(grep -Eo 'https://login\.tailscale\.com/a/[[:alnum:]]+' <<<"$output" | tail -1 || true)"
    [[ -n "$login_url" ]] && break
    sleep 1
done

echo "To authenticate, visit: $login_url" >&2
wait_for_ssh

echo "ubuntu@$machine_name"
