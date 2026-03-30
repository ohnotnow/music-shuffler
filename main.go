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
	cfg      config
	allFiles []string
	tracks   []string
	playing   int // -1 = nothing playing
	status    string
	err       string
	player    *exec.Cmd
	eqLevels  []int
	fullPaths bool
}

type playerFinishedMsg struct{ index int }
type eqTickMsg struct{}

var lastPlayedPath string

func loadConfig() config {
	cfg := config{
		TrackCount: defaultTrackCount,
		Player:     "afplay",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	paths := []string{
		"ms.yaml",
		filepath.Join(home, ".config", "ms", "config.yaml"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", p, err)
			continue
		}
		if cfg.TrackCount <= 0 || cfg.TrackCount > 10 {
			cfg.TrackCount = defaultTrackCount
		}
		return cfg
	}

	return cfg
}

func initialModel() model {
	cfg := loadConfig()
	m := model{
		cfg:      cfg,
		playing:  -1,
		eqLevels: make([]int, eqBars),
	}

	if len(cfg.MusicDirs) == 0 {
		m.err = "No music_dirs configured. Create ms.yaml or ~/.config/ms/config.yaml"
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	b.WriteString(helpStyle.Render(" 0-9 play · s stop · r shuffle · p path · q quit"))
	b.WriteString("\n")

	return b.String()
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if lastPlayedPath != "" {
		fmt.Printf("\nLast playing: %s\n", lastPlayedPath)
	}
}
