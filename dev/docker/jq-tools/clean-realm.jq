include "./remove-ids";
include "./sort-deep";

def remove_container_ids:
  remove_property(if type == "object" then del(.containerId) else . end);

def remove_builtin_flows:
  if type == "object" and has("authenticationFlows") then
    .authenticationFlows |= map(select(.builtIn != true))
  else
    .
  end;

# Remove all top level properties starting with "de.adorsys.keycloak.config.*"
def remove_import_properties:
  if type == "object" then
    with_entries(select(.key | startswith("de.adorsys.keycloak.config.") | not)) |
    with_entries(.value |= remove_import_properties)
  elif type == "array" then
    map(remove_import_properties)
  else
    .
  end;

# Function to restore $(env:ENV) placeholders for interpolated values
def restore_env_placeholders:
  if type == "object" then
    # OAuth providers
    if .providerId == "github" then
      .config.clientId = "$(env:GITHUB_OAUTH_CLIENT_ID)" |
      .config.clientSecret = "$(env:GITHUB_OAUTH_SECRET)"
    elif .providerId == "google" then
      .config.clientId = "$(env:GOOGLE_OAUTH_CLIENT_ID)" |
      .config.clientSecret = "$(env:GOOGLE_OAUTH_SECRET)"
    # Client configurations
    elif .clientId == "website" then
      .secret = "$(env:WEBSITE_CLIENT_SECRET)" |
      .redirectUris = ["$(env:WEBSITE_REDIRECT_URI)"]
    elif .clientId == "cli" then
      .secret = "$(env:XATA_CLI_CLIENT_SECRET)"
    elif .clientId == "frontend" then
      .secret = "$(env:FRONTEND_CLIENT_SECRET)" |
      .redirectUris = ["$(env:FRONTEND_REDIRECT_URI)"] |
      .webOrigins = ["$(env:FRONTEND_WEB_ORIGIN_URL)"] |
      .attributes["post.logout.redirect.uris"] = "$(env:FRONTEND_POST_LOGOUT_REDIRECT_URI)"
    elif .clientId == "mcp" then
      .secret = "$(env:MCP_CLIENT_SECRET)" |
      .redirectUris = ["$(env:MCP_REDIRECT_URI)"]
    elif .clientId == "account" then
      # Org-invite links are bound to the built-in account client; it needs the
      # app origin as a valid redirect so /login-actions/restart validation passes.
      .redirectUris = ["$(env:FRONTEND_REDIRECT_URI)", "/realms/xata/account/*"]
    # Turnstile configuration
    elif has("config") and .config.secret then
      .config.secret = "$(env:TURNSTILE_SECRET)" |
      .config."site.key" = "$(env:TURNSTILE_SITE_KEY)"
    # SMTP server
    elif has("smtpServer") then
      .smtpServer.from = "$(env:SMTP_FROM)" |
      .smtpServer.host = "$(env:SMTP_HOST)" |
      .smtpServer.password = "$(env:SMTP_PASSWORD)" |
      .smtpServer.port = "$(env:SMTP_PORT)" |
      .smtpServer.replyTo = "$(env:SMTP_REPLY_TO)" |
      .smtpServer.ssl = "$(env:SMTP_SSL)" |
      .smtpServer.user = "$(env:SMTP_USER)"
    else
      .
    end |
    # Recursively apply to nested objects
    with_entries(.value |= restore_env_placeholders)
  elif type == "array" then
    map(restore_env_placeholders)
  else
    .
  end;

remove_ids | sort_deep | remove_container_ids | remove_builtin_flows | restore_env_placeholders | remove_import_properties
