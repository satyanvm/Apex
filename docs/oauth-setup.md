# Setting up sign-in and one-click connections

Apex connects third-party accounts in one click when the server has an OAuth app
for that provider. This guide covers every provider the code supports. You only
need the ones your users will use; the rest keep the paste-a-token form.

Throughout, replace `n8ndag.xyz` with your domain if it differs (it must match
`APP_BASE_URL`).

## The two redirect URIs

| Purpose | Redirect URI |
|---|---|
| "Continue with Google" sign-in | `https://n8ndag.xyz/api/auth/google/callback` |
| Connecting any integration | `https://n8ndag.xyz/api/oauth/callback` |

Every provider app below uses the second one. Google's app uses both.

## How the variables are named

Each app's id and secret go in `.env` as `OAUTH_<PROVIDER>_CLIENT_ID` and
`OAUTH_<PROVIDER>_CLIENT_SECRET`. The provider name for each integration is in
the table at the end. Two exceptions:

- Google also accepts `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`, which sign-in uses too.
- To give one integration its own app, name it after the integration instead,
  e.g. `OAUTH_GMAIL_CLIENT_ID`. That wins over the provider-wide one.

After editing `.env`, recreate the services that read it:

```bash
docker compose -f docker-compose.prod.yml up -d --force-recreate api pieces reasoner
```

The API asks the pieces worker which apps exist and caches the answer for up to
an hour. Recreating the API clears it.

To check: open any workflow's Run dialog or the builder, pick a node for that
app, and click Create credential. A "Sign in with …" button means it's set up.

---

## Google (sign-in, Gmail, Sheets, Drive, Calendar, Docs, …)

One OAuth client covers sign-in and all 15 Google integrations.

1. Go to https://console.cloud.google.com and create a project (top bar → project picker → New project).
2. **Turn on the APIs** you'll use: APIs & Services → Library, then enable each of
   Gmail API, Google Sheets API, Google Drive API, Google Calendar API, Google Docs API,
   Google Slides API, Google Tasks API, People API (Contacts), Google Forms API,
   YouTube Data API v3, BigQuery API, Cloud Storage JSON API, Google Search Console API.
   A connection to an API that isn't enabled fails with a 403.
3. **Consent screen**: Google Auth Platform (or APIs & Services → OAuth consent screen) → Get started.
   - App name: Apex. User support email: yours.
   - Audience: **External**.
   - Contact email: yours. Agree and create.
   - Branding: add your home page (`https://n8ndag.xyz`), privacy policy and terms URLs,
     and under Authorized domains add `n8ndag.xyz`.
4. **Client**: Clients → Create client.
   - Application type: **Web application**. Name: Apex web.
   - Authorized JavaScript origins: `https://n8ndag.xyz`
   - Authorized redirect URIs: both URIs from the top of this guide.
   - Create, then copy the Client ID and Client secret.
5. In `.env`:
   ```
   GOOGLE_CLIENT_ID=…apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=GOCSPX-…
   ```
6. **Testing vs. production**: Audience → Test users → add the Google accounts that
   will connect Gmail/Sheets/Drive while the app is in Testing.
   - Sign-in alone (openid, email, profile) works for everyone with no review.
   - Gmail and full Drive are *restricted* scopes; Sheets and Calendar are *sensitive*.
     Opening them to all users means Publish app → verification, and for restricted
     scopes a yearly third-party security assessment.
   - In Testing, Google expires refresh tokens after **7 days**, so test connections
     need reconnecting weekly. Published apps don't have this limit.

## Telegram

No OAuth app. The site gets one bot that everyone connects to by tapping Start.

1. In Telegram, open **@BotFather** and send `/newbot`.
2. Give it a display name (Apex) and a username ending in `bot` (e.g. `n8ndag_apex_bot`).
3. Copy the token it replies with (`123456789:AA…`).
4. Optional: `/setdescription` and `/setuserpic` so users recognise it.
5. In `.env`: `TELEGRAM_BOT_TOKEN=123456789:AA…`
6. Don't set a webhook on this bot; Apex reads its Start messages directly.
   If you set one before, clear it:
   ```bash
   curl "https://api.telegram.org/bot<TOKEN>/deleteWebhook"
   ```

## Slack

1. https://api.slack.com/apps → Create New App → **From a manifest** → pick a workspace.
2. Paste the manifest below (YAML). It has the exact scopes the Slack integration asks for.

```yaml
display_information:
  name: Apex
features:
  bot_user:
    display_name: Apex
    always_online: false
oauth_config:
  redirect_urls:
    - https://n8ndag.xyz/api/oauth/callback
  scopes:
    bot:
      - channels:read
      - channels:manage
      - channels:history
      - chat:write
      - groups:read
      - groups:write
      - groups:history
      - reactions:read
      - mpim:read
      - mpim:write
      - mpim:history
      - im:write
      - im:read
      - im:history
      - users:read
      - files:write
      - files:read
      - users:read.email
      - reactions:write
      - usergroups:read
      - usergroups:write
      - chat:write.customize
      - links:read
      - links:write
      - emoji:read
      - users.profile:read
      - channels:write.invites
      - groups:write.invites
      - channels:join
      - conversations.connect:write
    user:
      - search:read
      - users.profile:write
      - reactions:read
      - reactions:write
      - im:history
      - stars:read
      - channels:write
      - groups:write
      - im:write
      - mpim:write
      - channels:write.invites
      - groups:write.invites
      - channels:history
      - groups:history
      - chat:write
      - users:read
      - usergroups:write
settings:
  org_deploy_enabled: false
  socket_mode_enabled: false
  token_rotation_enabled: false
```

3. Create. Basic Information → App Credentials: copy **Client ID** and **Client Secret**.
4. To let other workspaces connect: Manage Distribution → complete the checklist →
   Activate Public Distribution. Until then only your own workspace can connect.
5. `.env`: `OAUTH_SLACK_CLIENT_ID`, `OAUTH_SLACK_CLIENT_SECRET`.

## GitHub

1. https://github.com/settings/developers → OAuth Apps → New OAuth App.
   (For an organization: Organization settings → Developer settings → OAuth Apps.)
2. Homepage URL `https://n8ndag.xyz`. Authorization callback URL `https://n8ndag.xyz/api/oauth/callback`.
3. Register → copy Client ID → Generate a new client secret → copy it.
4. `.env`: `OAUTH_GITHUB_CLIENT_ID`, `OAUTH_GITHUB_CLIENT_SECRET`.

## Microsoft (Outlook, Teams, OneDrive, Excel, SharePoint, To Do, Planner, …)

One app registration covers all 15 Microsoft integrations.

1. https://entra.microsoft.com → Identity → Applications → **App registrations** → New registration.
   (Same thing in https://portal.azure.com → Microsoft Entra ID → App registrations.)
2. Name: Apex. Supported account types: **Accounts in any organizational directory and
   personal Microsoft accounts**. (The integrations sign in through the `common` tenant.)
3. Redirect URI: platform **Web**, `https://n8ndag.xyz/api/oauth/callback`. Register.
4. Overview → copy **Application (client) ID**.
5. Certificates & secrets → New client secret → copy the **Value** (not the Secret ID).
   It expires (24 months at most); put a reminder in your calendar.
6. You don't need to add API permissions by hand: each integration asks for its own
   scopes when the user connects. Some (Azure AD user management, SharePoint sites)
   need an admin of the user's organization to approve.
7. `.env`: `OAUTH_MICROSOFT_CLIENT_ID`, `OAUTH_MICROSOFT_CLIENT_SECRET`.

## Notion

1. https://www.notion.so/my-integrations → New integration.
2. Type: **Public**. Fill in company name, website, privacy policy and terms URLs.
3. Redirect URIs: `https://n8ndag.xyz/api/oauth/callback`. Save.
4. Configuration → copy **OAuth Client ID** and **OAuth Client Secret**.
5. `.env`: `OAUTH_NOTION_CLIENT_ID`, `OAUTH_NOTION_CLIENT_SECRET`.

## Dropbox

1. https://www.dropbox.com/developers/apps → Create app → Scoped access → Full Dropbox.
2. Settings → Redirect URIs: add `https://n8ndag.xyz/api/oauth/callback`.
3. **Permissions** tab: tick files.metadata.read/write and files.content.read/write → Submit.
   Dropbox rejects sign-ins that ask for scopes the app doesn't have.
4. Settings → copy **App key** (client id) and **App secret**.
5. `.env`: `OAUTH_DROPBOX_CLIENT_ID`, `OAUTH_DROPBOX_CLIENT_SECRET`.

## HubSpot

1. https://developers.hubspot.com → create a developer account → Apps → Create app.
2. Auth: add redirect URL `https://n8ndag.xyz/api/oauth/callback`.
3. Scopes: add every scope listed for HubSpot in the table below. HubSpot refuses a
   sign-in whose scopes don't match the app's.
4. Copy **Client ID** and **Client secret**.
5. `.env`: `OAUTH_HUBSPOT_CLIENT_ID`, `OAUTH_HUBSPOT_CLIENT_SECRET`.

HubSpot has been moving new apps to its "projects" developer platform; if the
Create app screen differs, the settings to find are the same three: redirect URL,
scopes, client id/secret.

## Facebook Pages, Facebook Leads, Instagram for Business

1. https://developers.facebook.com/apps → Create app → use case for managing
   business/Pages (app type **Business**).
2. Add the product **Facebook Login for Business** → Settings → Valid OAuth Redirect
   URIs: `https://n8ndag.xyz/api/oauth/callback`.
3. App settings → Basic: copy **App ID** and **App Secret**; add privacy policy URL.
4. Permissions such as pages_manage_posts and instagram_content_publish need App
   Review (and Business Verification) before people outside your app's roles can grant them.
5. `.env`: `OAUTH_FACEBOOK_CLIENT_ID`, `OAUTH_FACEBOOK_CLIENT_SECRET`.

## LinkedIn

1. https://www.linkedin.com/developers/apps → Create app (needs a LinkedIn Page).
2. Products: add **Sign In with LinkedIn using OpenID Connect** and **Share on LinkedIn**
   (organization posting needs the Community Management API, which LinkedIn reviews).
3. Auth → Authorized redirect URLs: `https://n8ndag.xyz/api/oauth/callback`.
4. Copy **Client ID** and **Primary Client Secret**.
5. `.env`: `OAUTH_LINKEDIN_CLIENT_ID`, `OAUTH_LINKEDIN_CLIENT_SECRET`.

## Salesforce

1. Setup → App Manager → **New Connected App** (newer orgs: New External Client App).
2. Enable OAuth Settings. Callback URL `https://n8ndag.xyz/api/oauth/callback`.
3. Scopes: Full access (full), Manage user data via APIs (api), Perform requests at any
   time (refresh_token, offline_access).
4. Save, wait about 10 minutes, then Manage Consumer Details → copy **Consumer Key**
   (client id) and **Consumer Secret**.
5. `.env`: `OAUTH_SALESFORCE_CLIENT_ID`, `OAUTH_SALESFORCE_CLIENT_SECRET`.

Users of sandbox orgs (test.salesforce.com) can't use one-click; it signs in at login.salesforce.com.

## Every other provider

The steps are the same everywhere:

1. Open the provider's developer portal (table below) and create an OAuth / API app.
2. Set its redirect (callback) URL to `https://n8ndag.xyz/api/oauth/callback`.
3. If the portal makes you pick scopes up front, pick the ones listed for that
   integration in the scopes table.
4. Copy its client id and client secret into `OAUTH_<PROVIDER>_CLIENT_ID` /
   `OAUTH_<PROVIDER>_CLIENT_SECRET`.

### Providers and variable prefixes

| Variable prefix | Integrations | Where to create the app |
|---|---|---|
| `OAUTH_ACUITYSCHEDULING_` | Acuity Scheduling | https://developers.acuityscheduling.com (OAuth2 → register a client) |
| `OAUTH_ASANA_` | Asana | https://app.asana.com/0/my-apps |
| `OAUTH_BAREMETRICS_` | Baremetrics | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_BEXIO_` | Bexio | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_BOX_` | Box | https://app.box.com/developers/console (Platform App → User Authentication (OAuth 2.0)) |
| `OAUTH_CAPSULECRM_` | Capsule CRM | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_CLICDATA_` | Clicdata | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_CLICKUP_` | ClickUp | ClickUp → Settings → Apps → Create new app |
| `OAUTH_CLOUDCONVERT_` | CloudConvert | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_CONSTANTCONTACT_` | Constant Contact | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_DIGITALOCEAN_` | DigitalOcean | https://cloud.digitalocean.com/account/api/applications |
| `OAUTH_DROPBOX_` | Dropbox | https://www.dropbox.com/developers/apps |
| `OAUTH_FACEBOOK_` | Facebook Leads, Facebook Pages, Instagram for Business | https://developers.facebook.com/apps |
| `OAUTH_FATHOM_` | Fathom | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_FIGMA_` | Figma | https://www.figma.com/developers/apps |
| `OAUTH_FORMSTACK_` | Formstack | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_FREEAGENT_` | FreeAgent | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_GETHARVEST_` | Harvest | https://id.getharvest.com/developers |
| `OAUTH_GETRESPONSE_` | GetResponse | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_GITEA_` | Gitea | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_GITHUB_` | GitHub | https://github.com/settings/developers |
| `OAUTH_GITLAB_` | GitLab | https://gitlab.com/-/user_settings/applications |
| `OAUTH_GOHIGHLEVEL_` | LeadConnector | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_GOOGLE_` | Gmail, Google BigQuery, Google Calendar, Google Chat, Google Cloud Storage, Google Contacts, Google Docs, Google Drive, Google Forms, Google My Business, Google Search Console, Google Sheets, Google Slides, Google Tasks, YouTube | https://console.cloud.google.com |
| `OAUTH_GOTENZO_` | Tenzo | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_GREENHOUSE_` | Greenhouse | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_HELPSCOUT_` | Help Scout | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_HOOTSUITE_` | Hootsuite | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_HUBSPOT_` | HubSpot | https://developers.hubspot.com |
| `OAUTH_INSTACHARTS_` | InstaCharts | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_INTUIT_` | Quickbooks Online | https://developer.intuit.com |
| `OAUTH_LIGHTFUNNELS_` | Lightfunnels | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_LINKEDIN_` | LinkedIn | https://www.linkedin.com/developers/apps |
| `OAUTH_MAILCHIMP_` | Mailchimp | https://admin.mailchimp.com/account/oauth2/ |
| `OAUTH_MICROSOFT_` | Azure Active Directory, Microsoft 365 Copilot, Microsoft 365 People, Microsoft 365 Planner, Microsoft Dynamics 365 Business Central, Microsoft Dynamics CRM, Microsoft Excel 365, Microsoft OneDrive, Microsoft OneNote, Microsoft Outlook, Microsoft Outlook Calendar, Microsoft Power BI, Microsoft SharePoint, Microsoft Teams, Microsoft To Do | https://entra.microsoft.com → App registrations |
| `OAUTH_MINDMEISTER_` | MeisterTask | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_MYCASE_` | MyCase | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_NETLIFY_` | Netlify | https://app.netlify.com/user/applications |
| `OAUTH_NIFTY_` | Nifty | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_NOTION_` | Notion | https://www.notion.so/my-integrations |
| `OAUTH_PINTEREST_` | Pinterest | https://developers.pinterest.com/apps |
| `OAUTH_PIPEDRIVE_` | Pipedrive | https://developers.pipedrive.com (Developer Hub) |
| `OAUTH_PODIO_` | Podio | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_RAINDROP_` | Raindrop | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_REDDIT_` | Reddit | https://www.reddit.com/prefs/apps (type: web app) |
| `OAUTH_SALESFORCE_` | Salesforce | Salesforce Setup → App Manager → New Connected App |
| `OAUTH_SAVVYCAL_` | SavvyCal | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_SIGNNOW_` | SignNow | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_SIMPLYPRINT_` | SimplyPrint | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_SLACK_` | Slack | https://api.slack.com/apps |
| `OAUTH_SPOTIFY_` | Spotify | https://developer.spotify.com/dashboard |
| `OAUTH_SQUAREUP_` | Square | https://developer.squareup.com/apps |
| `OAUTH_SURVEYMONKEY_` | SurveyMonkey | https://developer.surveymonkey.com/apps |
| `OAUTH_TEABLE_` | Teable | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_TEAMLEADER_` | Teamleader | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_TICKTICK_` | TickTick | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_TODOIST_` | Todoist | https://developer.todoist.com/appconsole.html |
| `OAUTH_TRUELAYER_` | TrueLayer | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_TWITCH_` | Twitch | https://dev.twitch.tv/console/apps |
| `OAUTH_TYPEFORM_` | Typeform | Typeform → Account settings → Developer apps |
| `OAUTH_UC_TECHNOLOGIES_` | ConnectUC | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_VIDEOASK_` | VideoAsk | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_VIMEO_` | Vimeo | https://developer.vimeo.com/apps |
| `OAUTH_WEBEXAPIS_` | Cisco Webex Meetings | https://developer.webex.com/my-apps |
| `OAUTH_WEBFLOW_` | Webflow | Webflow → Workspace settings → Apps & Integrations → Develop |
| `OAUTH_WEEKDONE_` | Weekdone | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_WRIKE_` | Wrike | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_XERO_` | Xero | https://developer.xero.com/app/manage |
| `OAUTH_ZOHO_` | Bigin by Zoho CRM | The provider's developer portal (search "<name> OAuth app") |
| `OAUTH_ZOOM_` | Zoom | https://marketplace.zoom.us → Develop → Build App |

### Not one-click

HTTP (OAuth2), Intercom, Snowflake, Workday, Zoho Bookings, Zoho Books, Zoho CRM, Zoho Campaigns, Zoho Desk, Zoho Invoice, Zoho Mail need a region, domain or account URL chosen per user, so they always use the
paste-a-token form.

### Scopes each integration asks for

| Integration | Piece | Prefix | Scopes |
|---|---|---|---|
| Acuity Scheduling | `acuity-scheduling` | `OAUTH_ACUITYSCHEDULING_` | api-v1 |
| Asana | `asana` | `OAUTH_ASANA_` | default |
| Baremetrics | `baremetrics` | `OAUTH_BAREMETRICS_` | read write |
| Bexio | `bexio` | `OAUTH_BEXIO_` | openid profile email offline_access contact_show contact_edit article_show article_edit kb_invoice_show kb_invoice_edit kb_offer_show kb_offer_edit kb_order_show kb_order_edit project_show project_edit task_show monitoring_edit accounting file |
| Box | `box` | `OAUTH_BOX_` | manage_webhook root_readonly root_readwrite |
| Capsule CRM | `capsule-crm` | `OAUTH_CAPSULECRM_` | read write |
| Clicdata | `clicdata` | `OAUTH_CLICDATA_` | account_read data_read data_write table_read table_write |
| ClickUp | `clickup` | `OAUTH_CLICKUP_` | (none) |
| CloudConvert | `cloudconvert` | `OAUTH_CLOUDCONVERT_` | task.read task.write |
| Constant Contact | `constant-contact` | `OAUTH_CONSTANTCONTACT_` | contact_data |
| DigitalOcean | `digital-ocean` | `OAUTH_DIGITALOCEAN_` | (none) |
| Dropbox | `dropbox` | `OAUTH_DROPBOX_` | files.metadata.write files.metadata.read files.content.write files.content.read |
| Facebook Leads | `facebook-leads` | `OAUTH_FACEBOOK_` | pages_show_list pages_manage_ads leads_retrieval pages_manage_metadata business_management |
| Facebook Pages | `facebook-pages` | `OAUTH_FACEBOOK_` | pages_show_list pages_manage_posts business_management pages_read_engagement |
| Instagram for Business | `instagram-business` | `OAUTH_FACEBOOK_` | instagram_basic instagram_content_publish business_management pages_show_list |
| Fathom | `fathom` | `OAUTH_FATHOM_` | public_api |
| Figma | `figma` | `OAUTH_FIGMA_` | file_content:read file_metadata:read file_comments:read file_comments:write |
| Formstack | `formstack` | `OAUTH_FORMSTACK_` | (none) |
| FreeAgent | `free-agent` | `OAUTH_FREEAGENT_` | (none) |
| Harvest | `harvest` | `OAUTH_GETHARVEST_` | harvest:all |
| GetResponse | `getresponse` | `OAUTH_GETRESPONSE_` | (none) |
| Gitea | `gitea` | `OAUTH_GITEA_` | read:repository write:repository read:issue write:issue |
| GitHub | `github` | `OAUTH_GITHUB_` | admin:repo_hook admin:org repo gist |
| GitLab | `gitlab` | `OAUTH_GITLAB_` | api read_user |
| LeadConnector | `lead-connector` | `OAUTH_GOHIGHLEVEL_` | campaigns.readonly contacts.write contacts.readonly locations.readonly locations/tags.readonly locations/tags.write opportunities.readonly opportunities.write users.readonly workflows.readonly forms.readonly |
| Gmail | `gmail` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/gmail.send https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.compose email |
| Google BigQuery | `google-bigquery` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/bigquery |
| Google Calendar | `google-calendar` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/calendar.readonly |
| Google Chat | `googlechat` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/chat.messages https://www.googleapis.com/auth/chat.spaces https://www.googleapis.com/auth/chat.memberships https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/directory.readonly |
| Google Cloud Storage | `google-cloud-storage` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/devstorage.read_write https://www.googleapis.com/auth/devstorage.full_control https://www.googleapis.com/auth/cloud-platform.read-only https://www.googleapis.com/auth/pubsub |
| Google Contacts | `google-contacts` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/contacts |
| Google Docs | `google-docs` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/documents https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/drive |
| Google Drive | `google-drive` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/drive |
| Google Forms | `google-forms` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/forms.responses.readonly https://www.googleapis.com/auth/drive.readonly |
| Google My Business | `google-my-business` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/business.manage |
| Google Search Console | `google-search-console` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/webmasters |
| Google Sheets | `google-sheets` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/spreadsheets https://www.googleapis.com/auth/drive.readonly https://www.googleapis.com/auth/drive |
| Google Slides | `google-slides` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/presentations https://www.googleapis.com/auth/drive https://www.googleapis.com/auth/spreadsheets |
| Google Tasks | `google-tasks` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/tasks |
| YouTube | `youtube` | `OAUTH_GOOGLE_` | https://www.googleapis.com/auth/youtube https://www.googleapis.com/auth/youtube.readonly https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.force-ssl |
| Tenzo | `tenzo` | `OAUTH_GOTENZO_` | tenzo/area:read tenzo/location:read tenzo/sales:read tenzo/sales/day:read tenzo/sales/day/location:read tenzo/sales/day/area:read tenzo/sales/product:read tenzo/sales/product/category:read tenzo/sales/product/item:read tenzo/sales/product/location:read tenzo/sales/product/area:read tenzo/sales/order:read tenzo/sales/order/location:read tenzo/sales/order/area:read tenzo/sales/time_of_day:read tenzo/sales/time_of_day/location:read tenzo/sales/time_of_day/area:read tenzo/forecast:read tenzo/forecast/day/location:read tenzo/forecast/time_of_day/location:read tenzo/forecast/product/location:read tenzo/schedule/calendar:read tenzo/schedule/calendar:write tenzo/schedule/event:read tenzo/schedule/event:write tenzo/analytics:read |
| Greenhouse | `greenhouse` | `OAUTH_GREENHOUSE_` | harvest:candidates:list harvest:candidates:create harvest:candidates:update harvest:applications:list harvest:jobs:list harvest:job_posts:list harvest:interviews:list harvest:scorecards:list harvest:notes:create |
| Help Scout | `help-scout` | `OAUTH_HELPSCOUT_` | (none) |
| Hootsuite | `hootsuite` | `OAUTH_HOOTSUITE_` | offline post |
| HubSpot | `hubspot` | `OAUTH_HUBSPOT_` | crm.lists.read crm.lists.write crm.objects.companies.read crm.objects.companies.write crm.objects.contacts.read crm.objects.contacts.write crm.objects.custom.read crm.objects.custom.write crm.objects.deals.read crm.objects.deals.write crm.objects.line_items.read crm.objects.owners.read crm.objects.leads.read crm.objects.leads.write crm.schemas.companies.read crm.schemas.contacts.read crm.schemas.custom.read crm.schemas.deals.read crm.schemas.line_items.read automation e-commerce tickets content settings.currencies.read settings.users.read settings.users.teams.read files forms scheduler.meetings.meeting-link.read |
| InstaCharts | `insta-charts` | `OAUTH_INSTACHARTS_` | read write |
| Quickbooks Online | `quickbooks` | `OAUTH_INTUIT_` | com.intuit.quickbooks.accounting |
| Lightfunnels | `lightfunnels` | `OAUTH_LIGHTFUNNELS_` | products,orders,customers,funnels |
| LinkedIn | `linkedin` | `OAUTH_LINKEDIN_` | w_member_social w_organization_social rw_organization_admin openid email profile |
| Mailchimp | `mailchimp` | `OAUTH_MAILCHIMP_` | (none) |
| Azure Active Directory | `azure-ad` | `OAUTH_MICROSOFT_` | User.Read.All User.ReadWrite.All Group.ReadWrite.All Directory.Read.All LicenseAssignment.ReadWrite.All User.RevokeSessions.All offline_access |
| Microsoft 365 Copilot | `microsoft-copilot` | `OAUTH_MICROSOFT_` | Sites.Read.All Mail.Read Files.ReadWrite.All People.Read.All OnlineMeetingTranscript.Read.All Chat.Read ChannelMessage.Read.All ExternalItem.Read.All AiEnterpriseInteraction.Read Files.Read.All |
| Microsoft 365 People | `microsoft-365-people` | `OAUTH_MICROSOFT_` | Contacts.ReadWrite offline_access |
| Microsoft 365 Planner | `microsoft-365-planner` | `OAUTH_MICROSOFT_` | Tasks.ReadWrite User.Read Group.Read.All |
| Microsoft Dynamics 365 Business Central | `microsoft-dynamics-365-business-central` | `OAUTH_MICROSOFT_` | https://api.businesscentral.dynamics.com/user_impersonation https://api.businesscentral.dynamics.com/Financials.ReadWrite.All |
| Microsoft Dynamics CRM | `microsoft-dynamics-crm` | `OAUTH_MICROSOFT_` | {hostUrl}/.default openid email profile offline_access |
| Microsoft Excel 365 | `microsoft-excel-365` | `OAUTH_MICROSOFT_` | Files.ReadWrite Sites.ReadWrite.All offline_access |
| Microsoft OneDrive | `microsoft-onedrive` | `OAUTH_MICROSOFT_` | Files.ReadWrite offline_access |
| Microsoft OneNote | `microsoft-onenote` | `OAUTH_MICROSOFT_` | Notes.ReadWrite User.Read offline_access |
| Microsoft Outlook | `microsoft-outlook` | `OAUTH_MICROSOFT_` | Mail.ReadWrite Mail.Send Calendars.Read offline_access User.Read |
| Microsoft Outlook Calendar | `microsoft-outlook-calendar` | `OAUTH_MICROSOFT_` | User.Read Calendars.ReadWrite offline_access |
| Microsoft Power BI | `microsoft-power-bi` | `OAUTH_MICROSOFT_` | https://analysis.windows.net/powerbi/api/Dataset.ReadWrite.All offline_access |
| Microsoft SharePoint | `microsoft-sharepoint` | `OAUTH_MICROSOFT_` | openid email profile offline_access Sites.Read.All Sites.ReadWrite.All Sites.Manage.All Files.ReadWrite.All |
| Microsoft Teams | `microsoft-teams` | `OAUTH_MICROSOFT_` | openid email profile offline_access User.Read Channel.Create Channel.ReadBasic.All ChannelMessage.Send Team.ReadBasic.All Chat.ReadWrite ChannelMessage.Read.All TeamMember.Read.All User.ReadBasic.All Presence.Read.All OnlineMeetingTranscript.Read.All OnlineMeetingRecording.Read.All |
| Microsoft To Do | `microsoft-todo` | `OAUTH_MICROSOFT_` | Tasks.ReadWrite User.Read offline_access |
| MeisterTask | `meistertask` | `OAUTH_MINDMEISTER_` | userinfo.profile userinfo.email meistertask |
| MyCase | `mycase-piece` | `OAUTH_MYCASE_` | (none) |
| Netlify | `netlify` | `OAUTH_NETLIFY_` | (none) |
| Nifty | `nifty` | `OAUTH_NIFTY_` | task project subtask milestone subteam |
| Notion | `notion` | `OAUTH_NOTION_` | (none) |
| Pinterest | `pinterest` | `OAUTH_PINTEREST_` | ads:read boards:read boards:write boards:read_secret pins:read pins:write pins:read_secret user_accounts:read |
| Pipedrive | `pipedrive` | `OAUTH_PIPEDRIVE_` | base admin contacts:full users:read deals:full activities:full leads:full products:full webhooks:full |
| Podio | `podio` | `OAUTH_PODIO_` | global:read global:write |
| Raindrop | `raindrop` | `OAUTH_RAINDROP_` | (none) |
| Reddit | `reddit` | `OAUTH_REDDIT_` | identity read submit edit history flair |
| Salesforce | `salesforce` | `OAUTH_SALESFORCE_` | refresh_token full api |
| SavvyCal | `savvycal` | `OAUTH_SAVVYCAL_` | (none) |
| SignNow | `sign-now` | `OAUTH_SIGNNOW_` | * |
| SimplyPrint | `simplyprint` | `OAUTH_SIMPLYPRINT_` | user.read printers.read printers.write printers.actions queue.read queue.write files.read files.write files.temp_upload spools.read spools.write print_history.read print_history.write statistics.read custom_fields.read custom_fields.write tags.read tags.write webhooks.read webhooks.write |
| Slack | `slack` | `OAUTH_SLACK_` | channels:read channels:manage channels:history chat:write groups:read groups:write groups:history reactions:read mpim:read mpim:write mpim:history im:write im:read im:history users:read files:write files:read users:read.email reactions:write usergroups:read usergroups:write chat:write.customize links:read links:write emoji:read users.profile:read channels:write.invites groups:write.invites channels:join conversations.connect:write — user scopes: search:read,users.profile:write,reactions:read,reactions:write,im:history,stars:read,channels:write,groups:write,im:write,mpim:write,channels:write.invites,groups:write.invites,channels:history,groups:history,chat:write,users:read,usergroups:write |
| Spotify | `spotify` | `OAUTH_SPOTIFY_` | user-read-playback-state user-modify-playback-state user-read-currently-playing user-read-playback-position user-read-recently-played playlist-read-private playlist-read-collaborative playlist-modify-private playlist-modify-public user-library-read user-library-modify user-follow-read user-follow-modify |
| Square | `square` | `OAUTH_SQUAREUP_` | MERCHANT_PROFILE_READ CUSTOMERS_READ CUSTOMERS_WRITE ITEMS_READ ITEMS_WRITE ORDERS_READ ORDERS_WRITE PAYMENTS_READ INVOICES_READ APPOINTMENTS_READ APPOINTMENTS_WRITE |
| SurveyMonkey | `surveymonkey` | `OAUTH_SURVEYMONKEY_` | responses_read responses_read_detail webhooks_read webhooks_write surveys_read |
| Teable | `teable` | `OAUTH_TEABLE_` | base|read table|read field|read record|read record|create record|update record|delete |
| Teamleader | `teamleader` | `OAUTH_TEAMLEADER_` | (none) |
| TickTick | `ticktick` | `OAUTH_TICKTICK_` | tasks:read tasks:write |
| Todoist | `todoist` | `OAUTH_TODOIST_` | data:read_write |
| TrueLayer | `truelayer` | `OAUTH_TRUELAYER_` | info accounts balance cards transactions direct_debits standing_orders offline_access signupplus verification |
| Twitch | `twitch` | `OAUTH_TWITCH_` | (none) |
| Typeform | `typeform` | `OAUTH_TYPEFORM_` | webhooks:write forms:read |
| ConnectUC | `connectuc` | `OAUTH_UC_TECHNOLOGIES_` | offline_access |
| VideoAsk | `videoask` | `OAUTH_VIDEOASK_` | (none) |
| Vimeo | `vimeo` | `OAUTH_VIMEO_` | public private edit upload delete interact |
| Cisco Webex Meetings | `webex` | `OAUTH_WEBEXAPIS_` | spark:all meeting:schedules_read |
| Webflow | `webflow` | `OAUTH_WEBFLOW_` | webhooks:write forms:read |
| Weekdone | `week-done` | `OAUTH_WEEKDONE_` | (none) |
| Weekdone | `weekdone` | `OAUTH_WEEKDONE_` | (none) |
| Wrike | `wrike` | `OAUTH_WRIKE_` | (none) |
| Xero | `xero` | `OAUTH_XERO_` | openid profile email offline_access accounting.contacts accounting.transactions accounting.reports.read accounting.journals.read accounting.budgets.read accounting.attachments accounting.settings projects |
| Bigin by Zoho CRM | `bigin-by-zoho` | `OAUTH_ZOHO_` | ZohoBigin.modules.ALL ZohoBigin.settings.ALL ZohoBigin.users.ALL ZohoBigin.notifications.ALL ZohoSearch.securesearch.READ |
| Zoom | `zoom` | `OAUTH_ZOOM_` | (none) |

This list comes from the integration packages installed in the pieces worker. If
you upgrade them, an integration's scopes or URLs can change.
