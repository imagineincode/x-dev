![GitHub Release](https://img.shields.io/github/v/release/imagineincode/x-dev) ![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/imagineincode/x-dev/go.yml)
 ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/imagineincode/x-dev) ![Static Badge](https://img.shields.io/badge/X_API-v2-green)

# X-yapper

    \ \  //
     \ \//
      \ \
     //\ \
    //  \ \
     yapper
X-Yapper is a CLI tool that allows you to send tweets to your x.com (Twitter) profile.

![X (formerly Twitter) Follow](https://img.shields.io/twitter/follow/imagineincode)

## Contents

1. [Authentication](#authentication)
1. [Prior to starting](#prior-to-starting)
    1. [X Developer App](#x-developer-app)
    1. [Local env variables](#local-environment-variables)
    1. [Building from source](#building-from-source)
    1. [Verify binary using GPG](#verify-binary-using-gpg)
1. [Using X-yapper](#using-x-yapper)
1. [Contact](#contact)

## Authentication

[OAuth 2.0 Authorization Code Flow with PKCE](https://developer.x.com/en/docs/authentication/oauth-2-0/authorization-code) is used to authenticate your X profile. The **Prior to Starting** section guides you through setting up the necessary tokens required to communicate through the [X API](https://developer.x.com/en/docs/x-api/getting-started/about-x-api).

## Prior to Starting

### X Developer App

To use x-yapper, you need an X Developer account (free). The steps below will guide you through the setup if you don't already have one.

1. Visit https://developer.x.com/ and sign-in using your X credentials.
2. Go through the initial setup steps to configure a new app through the developer portal.
3. After you complete the initial configuration (naming app, description, icon, etc.), click on the app through the Dashboard and scroll down to the  **User authentication set up** section and click **Edit**.

    <img src="./assets/img/user-auth-settings.png" width="600">

4. Set the following configurations:
    - **App Permissions**: Read and Write

        <img src="./assets/img/app-permissions.png" width="400">

    - **Type of app**: Native App

        <img src="./assets/img/type-of-app.png" width="400">

    - **App info**:
        - **Callback URL**: `http://localhost:8080/callback`
        - **Website URL**: `http://www.localhost`
    - The remaining configurations are optional.

        <img src="./assets/img/app-info.png" width="400">

    - Click **Save**.
5. At the top of the app page, click the **Keys and Tokens** tab.

    <img src="./assets/img/keys-and-tokens.png" width="200">

6. Create an **OAuth 2.0 Client ID and Client Secret**. Save these to a password manager as you will need to reference them later.

### Local Environment Variables

X-Yapper assumes you have the Client ID and Client Secret set as local environment variables. Follow the steps below based on your operating system:

> note: additional support for storing and/or referencing the Client ID and Client Secret will likely be added in a future release. This method was chosen for its ease of use and the nature of how the tokens are used.

#### Temporary (current session only)

##### Linux and macOS (Bash/Zsh)

```bash
export TWITTER_CLIENT_ID="<client_id>"
export TWITTER_CLIENT_SECRET="<client_secret>"
```

- **To verify:**

```bash
echo $TWITTER_CLIENT_ID
echo $TWITTER_CLIENT_SECRET
```

##### Windows

##### CMD

```cmd
set TWITTER_CLIENT_ID=<client_id>
set TWITTER_CLIENT_SECRET=<client_secret>
```

- **To verify:**

```cmd
echo %TWITTER_CLIENT_ID%
echo %TWITTER_CLIENT_SECRET%
```

##### PowerShell

```powershell
$env:TWITTER_CLIENT_ID="<client_id>"
$env:TWITTER_CLIENT_SECRET="<client_secret>"
```

- **To verify:**

```powershell
$env:TWITTER_CLIENT_ID
$env:TWITTER_CLIENT_SECRET
```

## Running X-Yapper

### Building from source

#### Prerequisites

1. Go 1.23 or higher
2. Git installed

#### Steps

##### Linux and macOS

1. Open a terminal window and clone the repo:

```bash
git clone https://github.com/imagineincode/x-dev.git
```

2. Navigate to the repo directory:

```bash
cd x-dev
```

3. Build the application:

```bash
go build -o x-yapper ./cmd/x-yapper
```

4. Run x-yapper:

```bash
./x-yapper
```

**Windows**

1. Open a terminal window and clone the repo:

```cmd
git clone https://github.com/imagineincode/x-dev.git
```

2. Navigate to the repo directory:

```cmd
cd x-dev
```

3. Build the application:

```cmd
go build -o x-yapper.exe ./cmd/x-yapper
```

4. Run x-yapper:

```cmd
.\x-yapper.exe
```

### Download Release

1. Navigate to the [Releases](https://github.com/imagineincode/x-dev/releases) section to download from GitHub.
2. Click on the release title to open its details page.
3. Under the **Assets** section, you’ll find:
   - The binary for your operating system or platform.
   - The signature file (`.asc`) for the binary.
   - The public key file (`x-yapper-cross-platform-public-key.asc`).
4. Download the binary, the corresponding `.asc` signature file, and the `x-yapper-cross-platform-public-key.asc` file.
5. Verify the binary using GPG:

```bash
gpg --import x-yapper-cross-platform-public-key.asc
gpg --verify <BINARY_FILE>.asc <BINARY_FILE>
```

### Download from CLI

1. curl - replace `<VERSION>` AND `<OS_PLATFORM>`.

```bash
curl -L -o x-yapper https://github.com/imagineincode/x-dev/releases/download/<VERSION>/<OS_PLATFORM>

chmod +x x-yapper
```

2. wget - replace `<VERSION>` AND `<OS_PLATFORM>`.

```bash
wget https://github.com/imagineincode/x-dev/releases/download/<VERSION>/<OS_PLATFORM>

chmod +x x-yapper
```

## Verify binary using GPG

Verifying the binary ensures that the file you downloaded has not been tampered with and is an authentic release signed by the developer. This process uses GPG (GNU Privacy Guard) to check the signature of the binary against the provided public key, confirming its integrity and origin.

1. Under the **Assets** section, you’ll find:
   - The signature file (`.asc`) for the binary.
   - The public key file (`x-yapper-cross-platform-public-key.asc`).
1. Download the corresponding `.asc` signature file and the `x-yapper-cross-platform-public-key.asc` file.
1. Verify the binary using GPG:

```bash
gpg --import x-yapper-cross-platform-public-key.asc

gpg --verify <BINARY_FILE>.asc <BINARY_FILE>
```

## Using X-Yapper

1. You'll be prompted to open an authentication link to enter you X account credentials.
2. Click the link (cmd + click), or copy/paste the link in your browser.

    <img src="./assets/img/authorize-app.png" width="500px">

3. Click **Authorize App**.
4. You should see the following message in the browser and can close the window:

```bash
Authorization successful! You can close this window.
```

5. Select **Start New Post**.
6. Your local text edit is opened within the terminal, allowing you to type your content. Save the file when done.
7. You'll be shown a preview of the post, with options to **Send Post** or **Discard**.
8. After clicking Send Post, you'll receive a confirmation message that the post was successful.
9. At this point, you can choose to send a new post or exit the app.

## Contact

- Email: [stephen@imagineincode.com](mailto:stephen@imagineincode.com)
