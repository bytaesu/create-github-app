# `create-github-app`

Create GitHub App OAuth credentials with a single command.

```bash
# npm
npx create-github-app

# pnpm
pnpm dlx create-github-app

# bun
bunx create-github-app

# yarn
yarn dlx create-github-app
```

## What it does

1. Opens a browser with a simple form
2. You enter an app name and callback URL
3. Redirects to GitHub to confirm creation
4. Returns `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`

## How it works

Uses the [GitHub App Manifest Flow](https://docs.github.com/en/apps/sharing-github-apps/registering-a-github-app-from-a-manifest) to programmatically create GitHub Apps.

## License

MIT
