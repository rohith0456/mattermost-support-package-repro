// Package seeder creates test content in a running Mattermost instance via REST API.
package seeder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Seeder creates demo content in a running Mattermost instance via REST API.
type Seeder struct {
	base   string
	client *http.Client
	token  string
	userID string
	teamID string
	chans  map[string]string // channel name → channel id
}

// Options controls what gets seeded.
type Options struct {
	SiteURL   string // e.g. http://localhost:8065
	Username  string // Mattermost admin username
	Password  string // Mattermost admin password
	PostCount int    // number of posts to create (default 20)
	WithFiles bool   // attach test images and text files
	Verbose   bool
}

// New creates a Seeder targeting the given base URL.
func New(base string) *Seeder {
	return &Seeder{
		base:   strings.TrimRight(base, "/"),
		client: &http.Client{Timeout: 30 * time.Second},
		chans:  make(map[string]string),
	}
}

// Run seeds the environment with test content.
func (s *Seeder) Run(opts Options) error {
	fmt.Printf("  → Connecting to %s ...\n", opts.SiteURL)

	if err := s.waitForServer(60); err != nil {
		return err
	}
	fmt.Printf("  ✓ Server reachable\n")

	// On a fresh install, the first POST to /api/v4/users creates the admin.
	// If the user already exists we get a 400/409 — that's fine, just ignore it.
	_ = s.createUser(opts.Username, opts.Password, opts.Username+"@repro.local")

	if err := s.login(opts.Username, opts.Password); err != nil {
		return fmt.Errorf(
			"login failed — open %s, complete the setup wizard to create an admin account, then re-run seed: %w",
			opts.SiteURL, err,
		)
	}
	fmt.Printf("  ✓ Logged in as %s\n", opts.Username)

	if err := s.ensureTeam(); err != nil {
		return fmt.Errorf("team setup: %w", err)
	}
	fmt.Printf("  ✓ Team ready\n")

	if err := s.resolveChannels(); err != nil {
		return fmt.Errorf("channel setup: %w", err)
	}
	fmt.Printf("  ✓ Channels ready (%d found)\n", len(s.chans))

	count, err := s.seedPosts(opts.PostCount, opts.WithFiles)
	if err != nil {
		return fmt.Errorf("seeding posts: %w", err)
	}
	fmt.Printf("  ✓ Created %d posts\n", count)

	return nil
}

// ─── server health ─────────────────────────────────────────────────────────

func (s *Seeder) waitForServer(maxSecs int) error {
	deadline := time.Now().Add(time.Duration(maxSecs) * time.Second)
	first := true
	for time.Now().Before(deadline) {
		resp, err := s.client.Get(s.base + "/api/v4/system/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				if !first {
					fmt.Println()
				}
				return nil
			}
		}
		if first {
			fmt.Print("  Waiting for server")
			first = false
		}
		fmt.Print(".")
		time.Sleep(2 * time.Second)
	}
	fmt.Println()
	return fmt.Errorf("Mattermost at %s not reachable after %d seconds — is `make run` still in progress?", s.base, maxSecs)
}

// ─── low-level HTTP helpers ─────────────────────────────────────────────────

func (s *Seeder) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, s.base+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	return s.client.Do(req)
}

func (s *Seeder) doJSON(method, path string, body, out interface{}) error {
	resp, err := s.do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %s %s → %d: %s", method, path, resp.StatusCode, string(b))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ─── auth ───────────────────────────────────────────────────────────────────

func (s *Seeder) createUser(username, password, email string) error {
	var u struct{ ID string `json:"id"` }
	return s.doJSON("POST", "/api/v4/users", map[string]string{
		"username": username,
		"password": password,
		"email":    email,
	}, &u)
}

func (s *Seeder) login(username, password string) error {
	resp, err := s.do("POST", "/api/v4/users/login", map[string]string{
		"login_id": username,
		"password": password,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login → %d: %s", resp.StatusCode, b)
	}
	s.token = resp.Header.Get("Token")
	var u struct {
		ID string `json:"id"`
	}
	return json.NewDecoder(resp.Body).Decode(&u)
}

// ─── team ───────────────────────────────────────────────────────────────────

func (s *Seeder) ensureTeam() error {
	var teams []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := s.doJSON("GET", "/api/v4/teams", nil, &teams); err != nil {
		return err
	}
	for _, t := range teams {
		s.teamID = t.ID
		return nil // use whichever team exists
	}
	// create one
	var team struct {
		ID string `json:"id"`
	}
	if err := s.doJSON("POST", "/api/v4/teams", map[string]string{
		"name":         "mm-repro",
		"display_name": "mm-repro",
		"type":         "O",
	}, &team); err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	s.teamID = team.ID
	return nil
}

// ─── channels ───────────────────────────────────────────────────────────────

func (s *Seeder) resolveChannels() error {
	var channels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := s.doJSON("GET", "/api/v4/teams/"+s.teamID+"/channels", nil, &channels); err != nil {
		return err
	}
	for _, c := range channels {
		s.chans[c.Name] = c.ID
	}
	// ensure off-topic exists
	for _, name := range []string{"town-square", "off-topic"} {
		if _, ok := s.chans[name]; !ok {
			var ch struct{ ID string `json:"id"` }
			if err := s.doJSON("POST", "/api/v4/channels", map[string]interface{}{
				"team_id":      s.teamID,
				"name":         name,
				"display_name": name,
				"type":         "O",
			}, &ch); err == nil {
				s.chans[name] = ch.ID
			}
		}
	}
	return nil
}

func (s *Seeder) chanID(name string) string {
	if id, ok := s.chans[name]; ok {
		return id
	}
	for _, id := range s.chans {
		return id
	}
	return ""
}

// ─── post helpers ────────────────────────────────────────────────────────────

func (s *Seeder) post(channelID, message, rootID string) (string, error) {
	payload := map[string]string{
		"channel_id": channelID,
		"message":    message,
	}
	if rootID != "" {
		payload["root_id"] = rootID
	}
	var p struct{ ID string `json:"id"` }
	if err := s.doJSON("POST", "/api/v4/posts", payload, &p); err != nil {
		return "", err
	}
	return p.ID, nil
}

func (s *Seeder) react(postID, emoji string) {
	_ = s.doJSON("POST", "/api/v4/reactions", map[string]string{
		"user_id":    s.userID,
		"post_id":    postID,
		"emoji_name": emoji,
	}, nil)
}

func (s *Seeder) postWithFiles(channelID, message string, fileIDs []string) (string, error) {
	payload := map[string]interface{}{
		"channel_id": channelID,
		"message":    message,
		"file_ids":   fileIDs,
	}
	var p struct{ ID string `json:"id"` }
	if err := s.doJSON("POST", "/api/v4/posts", payload, &p); err != nil {
		return "", err
	}
	return p.ID, nil
}

// ─── file upload ─────────────────────────────────────────────────────────────

func (s *Seeder) uploadFile(channelID, filename string, data []byte) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	_ = w.WriteField("channel_id", channelID)
	part, err := w.CreateFormFile("files", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequest("POST", s.base+"/api/v4/files", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		FileInfos []struct {
			ID string `json:"id"`
		} `json:"file_infos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.FileInfos) == 0 {
		return "", fmt.Errorf("no file info returned")
	}
	return result.FileInfos[0].ID, nil
}

// ─── image generation (no external deps) ─────────────────────────────────────

func generatePNG(r, g, b uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 480, 270))
	bg := color.RGBA{R: r, G: g, B: b, A: 255}
	accent := color.RGBA{R: 255, G: 255, B: 255, A: 80}
	for y := 0; y < 270; y++ {
		for x := 0; x < 480; x++ {
			if (x+y)%40 < 4 {
				img.Set(x, y, accent)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// ─── post templates ───────────────────────────────────────────────────────────

type postTemplate struct {
	channel string
	message string
}

var postTemplates = []postTemplate{
	{"town-square", "📣 Repro environment is up and running! Mattermost is healthy.\n\nThis environment was generated by **mm-repro** from a real support package."},
	{"town-square", "Quick heads-up: all credentials in this environment end with `_local_repro_only`. Nothing here touches production. 🔒"},
	{"town-square", "Verify search is working — type anything in the search bar at the top right. Results should appear instantly for database search, or via OpenSearch if configured."},
	{"town-square", "Here's a code block to test formatting:\n```bash\nmm-repro seed --project . --with-files\n```\nShould render with syntax highlighting."},
	{"town-square", "**Channels to explore:**\n- ~town-square (you're here)\n- ~off-topic\n\nAll channels are open — post freely for testing."},
	{"town-square", "Check out the [Mattermost Calls feature](https://mattermost.com/blog/mattermost-calls) — available as a built-in plugin. 🎙️"},
	{"town-square", "Testing `@here` notification — does the notification badge appear for all channel members? 👀"},
	{"town-square", "Quick test table:\n\n| Name | Role |\n|------|------|\n| Alice | Developer |\n| Bob | QA |\n| Carol | Lead |\n\nDoes the table render correctly?"},
	{"town-square", "Multi-line message test:\n\nLine 1: OK\nLine 2: OK\nLine 3: OK\n\nSpacing should be preserved above."},
	{"town-square", "🧪 Webhook test: in a real scenario, a webhook would POST here. For now this is a manual stand-in."},
	{"off-topic", "🎉 Off-topic channel works! Drop emojis here to test the emoji picker. 😄🚀🎸🦄🐱"},
	{"off-topic", "Long message test:\n\nThe quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog.\n\nWord wrap should kick in above."},
	{"off-topic", "Blockquote test:\n\n> This is a blockquote.\n> It should be indented and styled differently from regular text.\n\nDoes the formatting look correct?"},
	{"off-topic", "Link preview test — does this URL unfurl?\nhttps://github.com/rohith0456/mattermost-support-package-repro"},
	{"off-topic", "Formatting test: ~~strikethrough~~ **bold** *italic* `inline code`\n\nAll four should render distinctly."},
	{"off-topic", "React to this message with 👇 to confirm emoji reactions are working."},
	{"off-topic", "Numbered list:\n\n1. First item\n2. Second item\n3. Third item\n   - Nested bullet A\n   - Nested bullet B\n4. Fourth item"},
	{"off-topic", "Pin test: right-click this message → **Pin to channel**. Then check the pinned posts panel (📌 icon)."},
}

var emojiSet = []string{"thumbsup", "tada", "white_check_mark", "rocket", "fire", "eyes", "heart", "100", "wave", "muscle"}

// ─── main seed loop ───────────────────────────────────────────────────────────

func (s *Seeder) seedPosts(count int, withFiles bool) (int, error) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	templates := postTemplates
	// If count is larger than templates, pad with timestamped extras
	for len(templates) < count {
		extra := len(templates)
		templates = append(templates, postTemplate{
			channel: []string{"town-square", "off-topic"}[rng.Intn(2)],
			message: fmt.Sprintf("Auto-generated test post #%d 🧪 — %s", extra+1, time.Now().Format("15:04:05")),
		})
	}
	templates = templates[:count]

	// Track the first post in each channel for thread replies
	threadRoots := map[string]string{}
	created := 0

	// Pre-generate test images if needed
	imageColors := [][3]uint8{
		{0x16, 0x52, 0xCC}, // Mattermost blue
		{0x1e, 0x9e, 0x72}, // green
		{0xd2, 0x42, 0x30}, // red
	}
	imageIdx := 0

	for i, tmpl := range templates {
		chID := s.chanID(tmpl.channel)
		if chID == "" {
			continue
		}

		postID, err := s.post(chID, tmpl.message, "")
		if err != nil {
			continue
		}
		created++

		// Random reactions on ~30% of posts
		if rng.Intn(3) == 0 {
			s.react(postID, emojiSet[rng.Intn(len(emojiSet))])
		}
		// Double-react on ~15% of posts
		if rng.Intn(7) == 0 {
			s.react(postID, emojiSet[rng.Intn(len(emojiSet))])
		}

		// Store thread root for this channel (first post wins)
		if _, ok := threadRoots[tmpl.channel]; !ok {
			threadRoots[tmpl.channel] = postID
		}

		// Create a threaded conversation on every 5th post
		if i > 0 && i%5 == 0 {
			rootID := threadRoots[tmpl.channel]
			replies := []string{
				"Thread reply 1 — confirming threaded discussions work 🧵",
				"Thread reply 2 — each reply should nest under the parent.",
				"Thread reply 3 — click **View Thread** to see all replies in the sidebar.",
			}
			for _, reply := range replies {
				if rid, err := s.post(chID, reply, rootID); err == nil {
					created++
					s.react(rid, "thumbsup")
				}
			}
		}

		// Attach files to every ~5th post
		if withFiles && i > 0 && i%5 == 0 {
			c := imageColors[imageIdx%len(imageColors)]
			imageIdx++
			imgData := generatePNG(c[0], c[1], c[2])
			imgName := fmt.Sprintf("test-screenshot-%d.png", imageIdx)

			fileID, err := s.uploadFile(chID, imgName, imgData)
			if err == nil {
				msg := fmt.Sprintf("📸 Test image #%d attached — does the inline preview appear below?", imageIdx)
				if pid, err := s.postWithFiles(chID, msg, []string{fileID}); err == nil {
					created++
					s.react(pid, "eyes")
				}
			}

			// Also attach a text/log file
			logContent := fmt.Sprintf(
				"# mm-repro test log\nGenerated: %s\n\n[INFO]  Server started\n[INFO]  Database connected\n[WARN]  Cache miss — first request will be slower\n[INFO]  Plugin loaded: com.mattermost.calls\n[INFO]  Ready\n",
				time.Now().Format(time.RFC3339),
			)
			txtID, err := s.uploadFile(chID, "server.log", []byte(logContent))
			if err == nil {
				if pid, err := s.postWithFiles(chID, "📄 Log file attached — does the text preview load?", []string{txtID}); err == nil {
					created++
				_ = pid
				}
			}
		}
	}

	return created, nil
}
