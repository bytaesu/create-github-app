package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const (
	// Server
	port       = 3456
	timeoutMin = 5

	// GitHub API
	githubAPIVersion   = "2022-11-28"
	githubManifestAPI  = "https://api.github.com/app-manifests/%s/conversions"
	githubAppCreateURL = "https://github.com/settings/apps/new"

	// Defaults
	defaultCallbackURL = "http://localhost:3000/api/auth/callback/github"
	betterAuthURL      = "https://better-auth.com"
)

var redirectURL = fmt.Sprintf("http://localhost:%d/callback", port)

type GitHubAppCredentials struct {
	ID            int    `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	PEM           string `json:"pem"`
	WebhookSecret string `json:"webhook_secret"`
	HTMLURL       string `json:"html_url"`
}

func main() {
	printHeader()

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := startServer(codeChan, errChan)

	fmt.Println("  Opening browser...")
	openBrowser(fmt.Sprintf("http://localhost:%d", port))

	fmt.Println("  Waiting for authorization...")
	fmt.Println()

	select {
	case code := <-codeChan:
		fmt.Println("  Authorization received.")
		fmt.Println("  Exchanging code for credentials...")

		credentials, err := exchangeCode(code)
		if err != nil {
			fmt.Printf("\n  Error: %v\n\n", err)
			os.Exit(1)
		}

		printCredentials(credentials)

	case err := <-errChan:
		fmt.Printf("\n  Error: %v\n\n", err)
		os.Exit(1)

	case <-time.After(time.Duration(timeoutMin) * time.Minute):
		fmt.Println()
		fmt.Println("  Error: Timeout - no response received")
		fmt.Println()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func printHeader() {
	fmt.Println()
	fmt.Println("  ┌────────────────────────────┐")
	fmt.Println("  │        GitHub App          │")
	fmt.Println("  └────────────────────────────┘")
	fmt.Println()
}

// ANSI colors
const (
	reset = "\033[0m"
	dim   = "\033[2m"
	green = "\033[32m"
	cyan  = "\033[36m"
)

func printCredentials(c *GitHubAppCredentials) {
	fmt.Println()
	fmt.Printf("  %s✓%s GitHub App created successfully.\n", green, reset)
	fmt.Println()
	fmt.Printf("  %sName%s  %s\n", dim, reset, c.Name)
	fmt.Printf("  %sURL%s   %s\n", dim, reset, c.HTMLURL)
	fmt.Println()
	fmt.Printf("  %s# Add to .env%s\n", dim, reset)
	fmt.Println()
	fmt.Printf("  %sGITHUB_CLIENT_ID%s=%s\n", cyan, reset, c.ClientID)
	fmt.Printf("  %sGITHUB_CLIENT_SECRET%s=%s\n", cyan, reset, c.ClientSecret)
	fmt.Println()
	fmt.Printf("  %s# Configure Better Auth%s\n", dim, reset)
	fmt.Println()
	fmt.Printf("  %sexport const%s auth = %sbetterAuth%s({\n", dim, reset, cyan, reset)
	fmt.Printf("    %ssocialProviders%s: {\n", cyan, reset)
	fmt.Printf("      %sgithub%s: {\n", cyan, reset)
	fmt.Printf("        %sclientId%s: process.env.%sGITHUB_CLIENT_ID%s,\n", dim, reset, cyan, reset)
	fmt.Printf("        %sclientSecret%s: process.env.%sGITHUB_CLIENT_SECRET%s,\n", dim, reset, cyan, reset)
	fmt.Printf("      },\n")
	fmt.Printf("    },\n")
	fmt.Printf("  });\n")
	fmt.Println()
}

func generateState() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func startServer(codeChan chan<- string, errChan chan<- error) *http.Server {
	var expectedState string

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		expectedState = generateState()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(formPage(expectedState)))
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error: No code received"))
			errChan <- fmt.Errorf("no code received from GitHub")
			return
		}

		if state != expectedState {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error: Invalid state parameter"))
			errChan <- fmt.Errorf("invalid state parameter - possible CSRF attack")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(successPage()))
		codeChan <- code
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		fmt.Printf("  Server running at http://localhost:%d\n\n", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	return server
}

func exchangeCode(code string) (*GitHubAppCredentials, error) {
	url := fmt.Sprintf(githubManifestAPI, code)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API error: %d", resp.StatusCode)
	}

	var credentials GitHubAppCredentials
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		return nil, err
	}

	return &credentials, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}

	if cmd != nil {
		cmd.Start()
	}
}

func successPage() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Success</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: #000;
      color: #fff;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .container { text-align: center; padding: 48px; }
    .checkmark {
      width: 48px;
      height: 48px;
      border: 2px solid #fff;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 0 auto 24px;
    }
    .checkmark svg { width: 24px; height: 24px; }
    h1 { font-size: 24px; font-weight: 500; margin-bottom: 8px; letter-spacing: -0.5px; }
    p { color: #888; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="checkmark">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polyline points="20 6 9 17 4 12"></polyline>
      </svg>
    </div>
    <h1>GitHub App Created</h1>
    <p>You can close this window and return to your terminal.</p>
  </div>
</body>
</html>`
}

func formPage(state string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Create GitHub App</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      background: #000;
      color: #fff;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
    }
    .container { width: 100%%; max-width: 400px; }
    .header { margin-bottom: 32px; }
    h1 { font-size: 24px; font-weight: 500; letter-spacing: -0.5px; margin-bottom: 8px; }
    .subtitle { color: #888; font-size: 14px; }
    .form-group { margin-bottom: 20px; }
    label { display: block; font-size: 13px; font-weight: 500; margin-bottom: 8px; color: #888; }
    input {
      width: 100%%;
      padding: 12px 14px;
      background: #111;
      border: 1px solid #333;
      border-radius: 8px;
      color: #fff;
      font-size: 14px;
      transition: border-color 0.15s, box-shadow 0.15s;
    }
    input:focus { outline: none; border-color: #fff; box-shadow: 0 0 0 1px #fff; }
    input::placeholder { color: #555; }
    .hint { font-size: 12px; color: #888; margin-top: 6px; }
    button {
      width: 100%%;
      padding: 12px 24px;
      background: #fff;
      color: #000;
      border: none;
      border-radius: 8px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      transition: background 0.15s, transform 0.1s;
      margin-top: 8px;
    }
    button:hover { background: #eee; }
    button:active { transform: scale(0.98); }
    .divider { height: 1px; background: #222; margin: 24px 0; }
    .footer { text-align: center; font-size: 12px; color: #555; }
    .footer a { color: #888; text-decoration: none; }
    .footer a:hover { color: #fff; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Create GitHub App</h1>
      <p class="subtitle">Generate OAuth credentials with zero effort</p>
    </div>

    <form id="manifest-form" method="post" action="%s">
      <div class="form-group">
        <label for="app-name">App Name</label>
        <input type="text" id="app-name" name="app-name" required autocomplete="off" spellcheck="false" placeholder="better-auth" value="better-auth-%d">
        <p class="hint">Must be unique across GitHub</p>
      </div>

      <div class="form-group">
        <label for="callback-url">Callback URL</label>
        <input type="url" id="callback-url" name="callback-url" required autocomplete="off" spellcheck="false" value="%s">
        <p class="hint">Where users are redirected after authentication</p>
      </div>

      <input type="hidden" name="state" value="%s">
      <input type="hidden" name="manifest" id="manifest-input">

      <button type="submit">Continue with GitHub</button>
    </form>

    <div class="divider"></div>

    <p class="footer">
      Built for <a href="%s" target="_blank">BETTER-AUTH</a>.
    </p>
  </div>

  <script>
    const form = document.getElementById('manifest-form');
    const appNameInput = document.getElementById('app-name');
    const callbackUrlInput = document.getElementById('callback-url');
    const manifestInput = document.getElementById('manifest-input');

    form.addEventListener('submit', () => {
      const manifest = {
        name: appNameInput.value,
        url: '%s',
        callback_urls: [callbackUrlInput.value],
        redirect_url: '%s',
        default_permissions: { emails: 'read' },
        public: true,
        request_oauth_on_install: true
      };
      manifestInput.value = JSON.stringify(manifest);
    });
  </script>
</body>
</html>`,
		githubAppCreateURL, // form action
		time.Now().Unix(),  // app name suffix
		defaultCallbackURL, // callback URL default
		state,              // hidden state
		betterAuthURL,      // footer link
		betterAuthURL,      // manifest url
		redirectURL,        // manifest redirect_url
	)
}
