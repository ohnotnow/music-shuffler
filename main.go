package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

const (
	defaultTrackCount = 10
	eqBars            = 12
	eqMaxHeight       = 7
	eqTickInterval    = 120 * time.Millisecond
)

type appMode int

const (
	modeMain appMode = iota
	modeIgnoreInput
	modeIgnoreConfirm
)

const exampleConfig = `# Music Shuffler configuration

music_dirs:
  - /path/to/your/music/

ignore_patterns:
  - audiobook
  - spoken.*word

# Number of tracks to show (max 10)
track_count: 10

# Command used to play audio files (default: afplay)
# Examples: mpv, aplay, ffplay -nodisp -autoexit
player: afplay
`

var configPath string

type config struct {
	MusicDirs      []string `yaml:"music_dirs"`
	IgnorePatterns []string `yaml:"ignore_patterns"`
	TrackCount     int      `yaml:"track_count"`
	Player         string   `yaml:"player"`
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	numberStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	trackStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	playingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	eqStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	eqDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
)

type model struct {
	cfg            config
	allFiles       []string
	tracks         []string
	playing        int // -1 = nothing playing
	status         string
	err            string
	player         *exec.Cmd
	eqLevels       []int
	fullPaths      bool
	mode           appMode
	ignoreInput    textinput.Model
	pendingPattern string
}

type playerFinishedMsg struct{ index int }
type eqTickMsg struct{}

var lastPlayedPath string

// loadConfig reads from ~/.config/ms/config.yaml. If the file doesn't exist,
// it creates it from the example config and returns created=true so main() can
// print help and exit.
func loadConfig() (cfg config, created bool) {
	cfg = config{
		TrackCount: defaultTrackCount,
		Player:     "afplay",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, false
	}

	configPath = filepath.Join(home, ".config", "ms", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist — create from example
		dir := filepath.Dir(configPath)
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", mkErr)
			return cfg, false
		}
		if wErr := os.WriteFile(configPath, []byte(exampleConfig), 0644); wErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", wErr)
			return cfg, false
		}
		return cfg, true
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", configPath, err)
		return cfg, false
	}
	if cfg.TrackCount <= 0 || cfg.TrackCount > 10 {
		cfg.TrackCount = defaultTrackCount
	}
	return cfg, false
}

func initialModel(cfg config) model {
	ti := textinput.New()
	ti.Placeholder = "e.g. Big Finish"
	ti.CharLimit = 200

	m := model{
		cfg:         cfg,
		playing:     -1,
		eqLevels:    make([]int, eqBars),
		mode:        modeMain,
		ignoreInput: ti,
	}

	if len(cfg.MusicDirs) == 0 {
		m.err = fmt.Sprintf("No music_dirs configured. Edit %s", configPath)
		return m
	}

	m.allFiles = scanFiles(cfg)
	if len(m.allFiles) == 0 {
		m.err = fmt.Sprintf("No music files found in %v", cfg.MusicDirs)
		return m
	}
	m.tracks = pickRandom(m.allFiles, cfg.TrackCount)
	return m
}

func scanFiles(cfg config) []string {
	ignoreRe := buildIgnoreRegex(cfg.IgnorePatterns)
	var files []string
	for _, dir := range cfg.MusicDirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if ignoreRe != nil && ignoreRe.MatchString(strings.ToLower(path)) {
				return nil
			}
			files = append(files, path)
			return nil
		})
	}
	return files
}

func buildIgnoreRegex(patterns []string) *regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	combined := strings.Join(patterns, "|")
	re, err := regexp.Compile("(?i)(" + combined + ")")
	if err != nil {
		return nil
	}
	return re
}

// saveIgnorePattern appends a pattern to the ignore_patterns list in the config
// file, preserving comments and formatting via the yaml.Node API.
func saveIgnorePattern(pattern string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	// doc is a Document node; its first child is the root mapping
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected config structure")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a mapping")
	}

	// Find the ignore_patterns key in the mapping
	var seqNode *yaml.Node
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "ignore_patterns" {
			seqNode = root.Content[i+1]
			break
		}
	}

	newItem := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: pattern,
	}

	if seqNode != nil && seqNode.Kind == yaml.SequenceNode {
		seqNode.Content = append(seqNode.Content, newItem)
	} else {
		// No ignore_patterns key — add one
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "ignore_patterns",
		}
		valNode := &yaml.Node{
			Kind:    yaml.SequenceNode,
			Tag:     "!!seq",
			Content: []*yaml.Node{newItem},
		}
		root.Content = append(root.Content, keyNode, valNode)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func pickRandom(files []string, n int) []string {
	if len(files) <= n {
		return files
	}
	perm := rand.Perm(len(files))
	picked := make([]string, n)
	for i := 0; i < n; i++ {
		picked[i] = files[perm[i]]
	}
	return picked
}

func trackName(cfg config, path string) string {
	for _, dir := range cfg.MusicDirs {
		if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return filepath.Base(path)
}

func eqTickCmd() tea.Cmd {
	return tea.Tick(eqTickInterval, func(time.Time) tea.Msg {
		return eqTickMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return nil
}

// countMatches returns how many of the current tracks would be hidden by pattern.
// Returns -1 if the pattern is invalid regex.
func countMatches(tracks []string, pattern string) int {
	if pattern == "" {
		return 0
	}
	re, err := regexp.Compile("(?i)(" + pattern + ")")
	if err != nil {
		return -1
	}
	n := 0
	for _, t := range tracks {
		if re.MatchString(t) {
			n++
		}
	}
	return n
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeIgnoreInput:
		return m.updateIgnoreInput(msg)
	case modeIgnoreConfirm:
		return m.updateIgnoreConfirm(msg)
	default:
		return m.updateMain(msg)
	}
}

func (m model) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "q", "ctrl+c":
			if m.playing >= 0 {
				lastPlayedPath = m.tracks[m.playing]
			}
			m.stopPlayer()
			return m, tea.Quit

		case "r":
			m.stopPlayer()
			m.tracks = pickRandom(m.allFiles, m.cfg.TrackCount)
			m.playing = -1
			m.status = "Shuffled!"
			return m, nil

		case "p", " ":
			m.fullPaths = !m.fullPaths
			return m, nil

		case "s":
			m.stopPlayer()
			m.playing = -1
			m.status = "Stopped"
			return m, nil

		case "i":
			m.mode = modeIgnoreInput
			m.ignoreInput.Reset()
			if m.playing >= 0 {
				rel := trackName(m.cfg, m.tracks[m.playing])
				if base := strings.SplitN(rel, string(filepath.Separator), 2)[0]; base != rel {
					m.ignoreInput.SetValue(base)
				}
			}
			m.ignoreInput.Focus()
			m.status = ""
			return m, m.ignoreInput.Cursor.BlinkCmd()

		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(key[0] - '0')
			if idx < len(m.tracks) {
				m.stopPlayer()
				m.playing = idx
				m.status = ""
				playerCmd := m.startPlayer(idx)
				return m, tea.Batch(playerCmd, eqTickCmd())
			}
		}

	case eqTickMsg:
		if m.playing >= 0 {
			for i := range m.eqLevels {
				m.eqLevels[i] = rand.IntN(eqMaxHeight + 1)
			}
			return m, eqTickCmd()
		}

	case playerFinishedMsg:
		if m.playing == msg.index {
			m.playing = -1
			m.status = "Finished"
			for i := range m.eqLevels {
				m.eqLevels[i] = 0
			}
		}
	}
	return m, nil
}

func (m model) updateIgnoreInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = modeMain
			m.ignoreInput.Blur()
			return m, nil
		case "enter":
			pattern := m.ignoreInput.Value()
			if pattern == "" {
				m.mode = modeMain
				m.ignoreInput.Blur()
				return m, nil
			}
			if _, err := regexp.Compile("(?i)(" + pattern + ")"); err != nil {
				// invalid pattern — stay in input mode, error shown by View()
				return m, nil
			}
			m.pendingPattern = pattern
			m.mode = modeIgnoreConfirm
			m.ignoreInput.Blur()
			return m, nil
		}
	case eqTickMsg:
		// Keep EQ animating while in input mode
		if m.playing >= 0 {
			for i := range m.eqLevels {
				m.eqLevels[i] = rand.IntN(eqMaxHeight + 1)
			}
			return m, eqTickCmd()
		}
		return m, nil
	case playerFinishedMsg:
		if m.playing == msg.index {
			m.playing = -1
			for i := range m.eqLevels {
				m.eqLevels[i] = 0
			}
		}
		return m, nil
	}

	// Delegate to textinput for all other messages (typing, cursor blink, etc.)
	var cmd tea.Cmd
	m.ignoreInput, cmd = m.ignoreInput.Update(msg)
	return m, cmd
}

func (m model) updateIgnoreConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			if err := saveIgnorePattern(m.pendingPattern); err != nil {
				m.err = fmt.Sprintf("Failed to save pattern: %v", err)
				m.mode = modeMain
				return m, nil
			}
			m.cfg.IgnorePatterns = append(m.cfg.IgnorePatterns, m.pendingPattern)
			m.allFiles = scanFiles(m.cfg)
			m.stopPlayer()
			m.tracks = pickRandom(m.allFiles, m.cfg.TrackCount)
			m.playing = -1
			m.status = "Pattern added, reshuffled!"
			m.mode = modeMain
			m.pendingPattern = ""
			return m, nil
		case "n", "esc":
			m.mode = modeMain
			m.pendingPattern = ""
			return m, nil
		}
	case eqTickMsg:
		if m.playing >= 0 {
			for i := range m.eqLevels {
				m.eqLevels[i] = rand.IntN(eqMaxHeight + 1)
			}
			return m, eqTickCmd()
		}
		return m, nil
	case playerFinishedMsg:
		if m.playing == msg.index {
			m.playing = -1
			for i := range m.eqLevels {
				m.eqLevels[i] = 0
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *model) startPlayer(idx int) tea.Cmd {
	m.player = exec.Command(m.cfg.Player, m.tracks[idx])
	if err := m.player.Start(); err != nil {
		m.err = fmt.Sprintf("Failed to play: %v", err)
		m.playing = -1
		return nil
	}
	proc := m.player
	return func() tea.Msg {
		_ = proc.Wait()
		return playerFinishedMsg{index: idx}
	}
}

func (m *model) stopPlayer() {
	if m.player != nil && m.player.Process != nil {
		_ = m.player.Process.Kill()
		_ = m.player.Wait()
		m.player = nil
	}
}

func renderEQ(levels []int) string {
	blocks := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇"}
	var b strings.Builder
	b.WriteString(" ")
	for i, level := range levels {
		block := blocks[level]
		if level >= 5 {
			b.WriteString(eqStyle.Render(block))
		} else {
			b.WriteString(eqDimStyle.Render(block))
		}
		if i < len(levels)-1 {
			b.WriteString(" ")
		}
	}
	return b.String()
}

func (m model) View() string {
	if m.err != "" {
		return errorStyle.Render(m.err) + "\n"
	}

	switch m.mode {
	case modeIgnoreInput:
		return m.viewIgnoreInput()
	case modeIgnoreConfirm:
		return m.viewIgnoreConfirm()
	default:
		return m.viewMain()
	}
}

func (m model) viewMain() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("~ Music Shuffler ~"))
	b.WriteString("\n")

	for i, track := range m.tracks {
		num := numberStyle.Render(fmt.Sprintf(" %d ", i))
		name := track
		if !m.fullPaths {
			name = trackName(m.cfg, track)
		}
		if i == m.playing {
			b.WriteString(num + " " + playingStyle.Render("▶ "+name))
		} else {
			b.WriteString(num + " " + trackStyle.Render("  "+name))
		}
		b.WriteString("\n")
	}

	if m.playing >= 0 {
		b.WriteString("\n" + renderEQ(m.eqLevels) + "\n")
	} else if m.status != "" {
		b.WriteString("\n " + m.status + "\n")
	}

	b.WriteString(helpStyle.Render(" 0-9 play · s stop · r shuffle · p path · i ignore · q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m model) viewIgnoreInput() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("~ Add Ignore Pattern ~"))
	b.WriteString("\n")

	// Show a reference track for context
	refIdx := m.playing
	if refIdx < 0 && len(m.tracks) > 0 {
		refIdx = 0
	}
	if refIdx >= 0 && refIdx < len(m.tracks) {
		b.WriteString(helpStyle.Render(" Reference: "))
		b.WriteString(trackStyle.Render(m.tracks[refIdx]))
		b.WriteString("\n\n")
	}

	b.WriteString(" Add ignore pattern: ")
	b.WriteString(m.ignoreInput.View())
	b.WriteString("\n")

	pattern := m.ignoreInput.Value()
	if pattern != "" {
		matches := countMatches(m.tracks, pattern)
		if matches < 0 {
			b.WriteString(" " + errorStyle.Render("invalid pattern"))
		} else {
			b.WriteString(helpStyle.Render(fmt.Sprintf(" Would hide %d of %d tracks", matches, len(m.tracks))))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" enter confirm · esc cancel"))
	b.WriteString("\n")

	return b.String()
}

func (m model) viewIgnoreConfirm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("~ Add Ignore Pattern ~"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf(" Add %s to ignore patterns? ", playingStyle.Render(m.pendingPattern)))
	b.WriteString(numberStyle.Render("y/n"))
	b.WriteString("\n")

	return b.String()
}

func main() {
	cfg, created := loadConfig()
	if created {
		fmt.Printf("Created config at %s\n", configPath)
		fmt.Println("Edit it to set your music directories, then run again.")
		return
	}

	if _, err := os.Stat("ms.yaml"); err == nil {
		fmt.Println("Note: found ms.yaml in the current directory — this is no longer used.")
		fmt.Printf("Config is now read from %s\n\n", configPath)
	}

	p := tea.NewProgram(initialModel(cfg))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if lastPlayedPath != "" {
		fmt.Printf("\nLast playing: %s\n", lastPlayedPath)
	}
}
