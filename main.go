package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	musicDir   = "/Volumes/Blah/itunes/"
	trackCount = 10
)

var ignorePatterns = []string{
	"agatha",
	"bbc",
	"spoken.*word",
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	numberStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	trackStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	playingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type model struct {
	allFiles []string
	tracks   []string
	playing  int // -1 = nothing playing
	status   string
	err      string
	player   *exec.Cmd
}

type playerFinishedMsg struct{ index int }

func initialModel() model {
	m := model{playing: -1}
	m.allFiles = scanFiles()
	if len(m.allFiles) == 0 {
		m.err = fmt.Sprintf("No music files found in %s", musicDir)
		return m
	}
	m.tracks = pickRandom(m.allFiles, trackCount)
	return m
}

func scanFiles() []string {
	ignoreRe := buildIgnoreRegex()
	var files []string
	_ = filepath.WalkDir(musicDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ignoreRe != nil && ignoreRe.MatchString(strings.ToLower(path)) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}

func buildIgnoreRegex() *regexp.Regexp {
	if len(ignorePatterns) == 0 {
		return nil
	}
	combined := strings.Join(ignorePatterns, "|")
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

func trackName(path string) string {
	rel, err := filepath.Rel(musicDir, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
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
			m.stopPlayer()
			return m, tea.Quit

		case "r":
			m.stopPlayer()
			m.tracks = pickRandom(m.allFiles, trackCount)
			m.playing = -1
			m.status = "Shuffled!"
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
				cmd := m.startPlayer(idx)
				return m, cmd
			}
		}

	case playerFinishedMsg:
		if m.playing == msg.index {
			m.playing = -1
			m.status = "Finished"
		}
	}
	return m, nil
}

func (m *model) startPlayer(idx int) tea.Cmd {
	m.player = exec.Command("afplay", m.tracks[idx])
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

func (m model) View() string {
	if m.err != "" {
		return errorStyle.Render(m.err) + "\n"
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("~ Music Shuffler ~"))
	b.WriteString("\n")

	for i, track := range m.tracks {
		num := numberStyle.Render(fmt.Sprintf(" %d ", i))
		name := trackName(track)
		if i == m.playing {
			b.WriteString(num + " " + playingStyle.Render("▶ "+name))
		} else {
			b.WriteString(num + " " + trackStyle.Render("  "+name))
		}
		b.WriteString("\n")
	}

	if m.status != "" {
		b.WriteString("\n " + m.status + "\n")
	}

	b.WriteString(helpStyle.Render(" 0-9 play · s stop · r shuffle · q quit"))
	b.WriteString("\n")

	return b.String()
}

func main() {
	if _, err := os.Stat(musicDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: music directory %q does not exist\n", musicDir)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
