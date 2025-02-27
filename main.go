package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"os/user"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	BaseDir     string
	MarkdownDir string
	MaxNoteLen  int
}

var defaultConfig = Config{
	MarkdownDir: "logs",
	MaxNoteLen:  1000,
}

func loadConfig() (*Config, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	config := defaultConfig
	config.BaseDir = filepath.Join(usr.HomeDir, ".dailylog")

	// TODO: Check for config file and override defaults
	return &config, nil
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#7D56F4"))
)

type mode int

const (
	normalMode mode = iota
	addMode
	viewMode
	searchMode
)

type item struct {
	title    string
	filepath string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.filepath }
func (i item) FilterValue() string { return i.title }

type model struct {
	list          list.Model
	textInput     textinput.Model
	viewport      viewport.Model
	mode          mode
	err           error
	width         int
	height        int
	currentFile   string
	config        *Config
	searchInput   textinput.Model
	searchResults []list.Item
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter your daily note..."
	ti.CharLimit = 156
	ti.Width = 50

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Daily Notes"
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		PaddingLeft(2).
		PaddingRight(2)

	return model{
		list:      l,
		textInput: ti,
		viewport:  vp,
		mode:      normalMode,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return loadFiles(m.config)
		},
		tea.EnterAltScreen,
	)
}

func loadFiles(config *Config) tea.Msg {
	var items []list.Item
	files, err := os.ReadDir(filepath.Join(config.BaseDir, config.MarkdownDir))
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w",
			filepath.Join(config.BaseDir, config.MarkdownDir), err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".md" {
			items = append(items, item{
				title:    file.Name(),
				filepath: filepath.Join(config.BaseDir, config.MarkdownDir, file.Name()),
			})
		}
	}
	for i := 0; i < len(items)/2; i++ {
		j := len(items) - 1 - i
		items[i], items[j] = items[j], items[i]
	}

	return items
}

func setViewportContent(m *model, content string) {
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.mode == addMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				m.textInput.Reset()
				m.textInput.Blur()
				return m, nil
			case "enter":
				if err := m.saveNote(m.textInput.Value()); err != nil {
					m.err = err
					return m, nil
				}
				m.textInput.Reset()
				m.mode = normalMode
				return m, func() tea.Msg {
					return loadFiles(m.config)
				}
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if m.mode == viewMode {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4
		} else {
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height - 4)
		}

		return m, nil

	case tea.KeyMsg:
		if m.mode == normalMode {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "a":
				m.mode = addMode
				m.textInput.Focus()
				return m, textinput.Blink
			case "B":
				if err := m.backup(); err != nil {
					m.err = err
					return m, nil
				}
				return m, nil
			case "enter":
				if i, ok := m.list.SelectedItem().(item); ok {
					content, err := os.ReadFile(i.filepath)
					if err != nil {
						m.err = err
						return m, nil
					}
					setViewportContent(&m, string(content))
					m.currentFile = i.filepath
					m.mode = viewMode
					m.viewport.Width = m.width
					m.viewport.Height = m.height - 4
					return m, nil
				}
			}
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		if m.mode == viewMode {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "esc", "h", "left":
				m.mode = normalMode
				return m, nil
			}
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case []list.Item:
		m.list.SetItems(msg)
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	return m, cmd
}

type ErrFileAccess struct {
	Op   string
	Path string
	Err  error
}

func (e *ErrFileAccess) Error() string {
	return fmt.Sprintf("failed to %s %s: %v", e.Op, e.Path, e.Err)
}

func (m model) saveNote(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	if len(content) > m.config.MaxNoteLen {
		return fmt.Errorf("note too long: maximum length is %d characters", m.config.MaxNoteLen)
	}

	now := time.Now()
	filename := now.Format("01-02-2006") + ".md"
	filePath := filepath.Join(m.config.BaseDir, m.config.MarkdownDir, filename)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return &ErrFileAccess{Op: "create directory", Path: filepath.Dir(filePath), Err: err}
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &ErrFileAccess{Op: "open", Path: filePath, Err: err}
	}
	defer f.Close()

	if err := lockFile(f); err != nil {
		return fmt.Errorf("failed to lock file: %w", err)
	}
	defer unlockFile(f)

	entry := fmt.Sprintf("- [%s] %s\n", now.Format("15:04"), content)
	if _, err := f.WriteString(entry); err != nil {
		return &ErrFileAccess{Op: "write", Path: filePath, Err: err}
	}

	return nil
}

func lockFile(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock file: %w", err)
	}
	return nil
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress any key to continue", m.err)
	}

	switch m.mode {
	case addMode:
		return fmt.Sprintf(
			"\n%s\n\n%s\n\n%s",
			titleStyle.Render("Add Daily Note"),
			m.textInput.View(),
			infoStyle.Render("(esc to cancel)"),
		)

	case viewMode:
		return fmt.Sprintf(
			"%s\n%s\n\n%s",
			titleStyle.Render("Viewing: "+filepath.Base(m.currentFile)),
			m.viewport.View(),
			infoStyle.Render("(h to go back)"),
		)

	default:
		return fmt.Sprintf(
			"%s\n\n%s",
			m.list.View(),
			infoStyle.Render("vim keys (h/j/k/l) • (ctrl+u/d) page up/down • (a) add note • (B) backup • (enter) view • (q) quit"),
		)
	}
}

func (m *model) search(query string) []list.Item {
	if query == "" {
		return m.list.Items()
	}

	var results []list.Item
	query = strings.ToLower(query)

	for _, listItem := range m.list.Items() {
		if noteItem, ok := listItem.(item); ok {
			content, err := os.ReadFile(noteItem.filepath)
			if err != nil {
				continue
			}

			if strings.Contains(strings.ToLower(string(content)), query) {
				results = append(results, listItem)
			}
		}
	}

	return results
}

func (m model) backup() error {
	backupDir := filepath.Join(m.config.BaseDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return &ErrFileAccess{Op: "create backup directory", Path: backupDir, Err: err}
	}

	timestamp := time.Now().Format("2006-01-02-150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup-%s.zip", timestamp))

	zipfile, err := os.Create(backupFile)
	if err != nil {
		return &ErrFileAccess{Op: "create backup file", Path: backupFile, Err: err}
	}
	defer zipfile.Close()

	zw := zip.NewWriter(zipfile)
	defer zw.Close()

	markdownPath := filepath.Join(m.config.BaseDir, m.config.MarkdownDir)
	err = filepath.Walk(markdownPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(m.config.BaseDir, path)
		if err != nil {
			return err
		}

		w, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = w.Write(content)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Join(config.BaseDir, config.MarkdownDir), 0755); err != nil {
		fmt.Printf("Error creating directories: %v\n", err)
		os.Exit(1)
	}

	m := initialModel()
	m.config = config

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
