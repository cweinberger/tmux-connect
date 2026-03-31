package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	// Colors — cyberpunk neon palette
	colorPrimary   = lipgloss.Color("#00E5FF") // electric cyan
	colorSecondary = lipgloss.Color("#BD00FF") // neon purple
	colorAccent    = lipgloss.Color("#39FF14") // neon green
	colorDim       = lipgloss.Color("#4A5568") // dark gray
	colorMuted     = lipgloss.Color("#718096") // muted gray
	colorBg        = lipgloss.Color("#0A0E17") // deep dark bg
	colorCardBg    = lipgloss.Color("#111827") // card bg
	colorHighlight = lipgloss.Color("#1A1F3D") // highlight bg
	colorWhite     = lipgloss.Color("#E2E8F0") // soft white
	colorRed       = lipgloss.Color("#FF0055") // hot pink-red
	colorYellow    = lipgloss.Color("#FFE600") // electric yellow
	colorCyan      = lipgloss.Color("#00E5FF") // cyan
	colorMagenta   = lipgloss.Color("#FF00FF") // magenta
	colorElecBlue  = lipgloss.Color("#0080FF") // electric blue
	colorHotPink   = lipgloss.Color("#FF2D95") // hot pink

	// Header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0A0E17")).
			Background(colorPrimary).
			Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Session list
	sessionBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(colorSecondary).
			Padding(0, 1).
			MarginTop(0).
			MarginBottom(0)

	sessionItemStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Padding(0, 1)

	sessionSelectedStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Background(colorHighlight).
				Padding(0, 1)

	sessionNameStyle = lipgloss.NewStyle().
				Foreground(colorWhite)

	sessionInfoStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	shortcutStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	attachedDot = lipgloss.NewStyle().
			Foreground(colorAccent).
			Render("\u25C9") // ◉

	detachedDot = lipgloss.NewStyle().
			Foreground(colorDim).
			Render("\u25CE") // ◎

	// Footer / help
	helpStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	// Status
	statusStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			MarginTop(1)

	// Quit menu
	quitBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorRed).
			Padding(1, 2).
			MarginTop(1)

	quitOptionStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Padding(0, 1)

	quitSelectedStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Background(lipgloss.Color("#1A0010")).
				Padding(0, 1)

	// Input
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorSecondary).
			Padding(0, 1).
			MarginTop(1)

	// Spinner
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorMagenta)

	// Window rows
	windowItemStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	windowSelectedStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Background(colorHighlight).
				Bold(true).
				Padding(0, 1)
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

type tmuxWindow struct {
	Index   int
	Name    string
	Active  bool
	Command string // current pane command
}

type tmuxSession struct {
	Name     string
	Windows  int
	Attached bool
	Activity string // current command in active pane
	WinList  []tmuxWindow
}

// Menu action IDs
const (
	menuOpenAll = iota
	menuKill
	menuNewWindow
	menuNew
	menuRefresh
	menuQuit
)

type menuItem struct {
	id    int
	label string
	key   string
}

type viewMode int

const (
	viewNormal viewMode = iota
	viewQuit
	viewNewSession
	viewConfirmKill
)

// visibleItem represents a single navigable row in the list
type visibleItem struct {
	kind       string // "session", "window", "menu"
	sessionIdx int
	windowIdx  int
	menuIdx    int
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type sessionsMsg struct {
	sessions []tmuxSession
	err      error
}

type tickMsg time.Time

type statusMsg string

type clearStatusMsg struct{}

type tabOpenedMsg struct {
	name string
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	remote    string
	useSSH    bool
	sessions  []tmuxSession
	cursor    int
	loading   bool
	spinner   spinner.Model
	status    string
	statusErr bool
	mode      viewMode
	quitIdx   int // 0 = just quit, 1 = close tabs & quit
	textInput textinput.Model
	terminal  string // "iterm2", "terminal", "other"
	width     int    // terminal width, defaults to 80
	height    int
	quote     string
	expanded  map[string]bool // tracks which sessions are expanded
}

func initialModel(remote string, useSSH bool) model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = spinnerStyle

	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.CharLimit = 64
	ti.Width = 30
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorMagenta)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorPrimary)

	return model{
		remote:    remote,
		useSSH:    useSSH,
		loading:   true,
		spinner:   s,
		terminal:  detectTerminal(),
		textInput: ti,
		width:     80,
		quote:     quotes[rand.Intn(len(quotes))],
		expanded:  make(map[string]bool),
	}
}

func (m model) menuItems() []menuItem {
	items := []menuItem{}
	items = append(items, menuItem{menuKill, "Kill selected", "x"})
	items = append(items, menuItem{menuNewWindow, "New window in session", "w"})
	if len(m.sessions) > 0 {
		items = append(items, menuItem{menuOpenAll, "Open all in tabs", "a"})
	}
	items = append(items,
		menuItem{menuNew, "New session", "n"},
		menuItem{menuRefresh, "Refresh", "r"},
		menuItem{menuQuit, "Quit", "q"},
	)
	return items
}

func (m model) visibleItems() []visibleItem {
	var items []visibleItem
	for i, s := range m.sessions {
		items = append(items, visibleItem{kind: "session", sessionIdx: i})
		if m.expanded[s.Name] {
			for j := range s.WinList {
				items = append(items, visibleItem{kind: "window", sessionIdx: i, windowIdx: j})
			}
		}
	}
	for i := range m.menuItems() {
		items = append(items, visibleItem{kind: "menu", menuIdx: i})
	}
	return items
}

func (m model) totalItems() int {
	return len(m.visibleItems())
}

func (m model) isKillEnabled() bool {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return false
	}
	item := items[m.cursor]
	// Kill is enabled if a session or one of its windows is selected
	return (item.kind == "session" || item.kind == "window") && len(m.sessions) > 0
}

func (m model) isOnKillItem() bool {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return false
	}
	item := items[m.cursor]
	if item.kind != "menu" {
		return false
	}
	menus := m.menuItems()
	if item.menuIdx < 0 || item.menuIdx >= len(menus) {
		return false
	}
	id := menus[item.menuIdx].id
	return id == menuKill || id == menuNewWindow
}

func (m model) isSessionSelected() bool {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return false
	}
	item := items[m.cursor]
	return item.kind == "session" || item.kind == "window"
}

// selectedSessionIdx returns the session index for the currently selected item (session or window).
// Returns -1 if a menu item is selected.
func (m model) selectedSessionIdx() int {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return -1
	}
	item := items[m.cursor]
	if item.kind == "session" || item.kind == "window" {
		return item.sessionIdx
	}
	return -1
}

func (m model) selectedMenuIdx() int {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return -1
	}
	item := items[m.cursor]
	if item.kind == "menu" {
		return item.menuIdx
	}
	return -1
}

// ---------------------------------------------------------------------------
// Terminal detection
// ---------------------------------------------------------------------------

func detectTerminal() string {
	termProgram := os.Getenv("TERM_PROGRAM")
	switch strings.ToLower(termProgram) {
	case "iterm.app":
		return "iterm2"
	case "apple_terminal":
		return "terminal"
	default:
		return "other"
	}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func fetchSessions(remote string) tea.Cmd {
	return func() tea.Msg {
		// Single SSH call: get sessions, then a delimiter, then windows for each session
		sshCmd := `tmux list-sessions -F '#{session_name}|#{session_windows}|#{session_attached}|#{pane_current_command}' 2>/dev/null; echo '---DELIM---'; for s in $(tmux list-sessions -F '#{session_name}' 2>/dev/null); do echo "SESSION:$s"; tmux list-windows -t "$s" -F '#{window_index}|#{window_name}|#{window_active}|#{pane_current_command}' 2>/dev/null; done`
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=5", remote, sshCmd)
		out, err := cmd.Output()
		if err != nil {
			return sessionsMsg{err: fmt.Errorf("connection failed: %w", err)}
		}

		output := strings.TrimSpace(string(out))
		parts := strings.SplitN(output, "---DELIM---", 2)

		// Parse sessions from the first part
		var sessions []tmuxSession
		sessionIdxMap := make(map[string]int)
		if len(parts) >= 1 {
			lines := strings.Split(strings.TrimSpace(parts[0]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				fields := strings.SplitN(line, "|", 4)
				if len(fields) < 3 {
					continue
				}
				windows, _ := strconv.Atoi(fields[1])
				attached := fields[2] != "0"
				activity := ""
				if len(fields) >= 4 {
					activity = fields[3]
				}
				sessionIdxMap[fields[0]] = len(sessions)
				sessions = append(sessions, tmuxSession{
					Name:     fields[0],
					Windows:  windows,
					Attached: attached,
					Activity: activity,
				})
			}
		}

		// Parse windows from the second part
		if len(parts) >= 2 {
			winLines := strings.Split(strings.TrimSpace(parts[1]), "\n")
			currentIdx := -1
			for _, line := range winLines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "SESSION:") {
					sessName := strings.TrimPrefix(line, "SESSION:")
					if idx, ok := sessionIdxMap[sessName]; ok {
						currentIdx = idx
					} else {
						currentIdx = -1
					}
					continue
				}
				if currentIdx < 0 {
					continue
				}
				fields := strings.SplitN(line, "|", 4)
				if len(fields) < 3 {
					continue
				}
				idx, _ := strconv.Atoi(fields[0])
				active := fields[2] != "0"
				winCmd := ""
				if len(fields) >= 4 {
					winCmd = fields[3]
				}
				sessions[currentIdx].WinList = append(sessions[currentIdx].WinList, tmuxWindow{
					Index:   idx,
					Name:    fields[1],
					Active:  active,
					Command: winCmd,
				})
			}
		}

		return sessionsMsg{sessions: sessions}
	}
}

func killWindow(remote, sessionName string, windowIndex int) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=5", remote,
			fmt.Sprintf("tmux kill-window -t %s:%d", sessionName, windowIndex))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return sessionsMsg{err: fmt.Errorf("failed to kill window: %w: %s", err, string(out))}
		}
		return statusMsg(fmt.Sprintf("Killed window %d in '%s'", windowIndex, sessionName))
	}
}

func createWindow(remote, sessionName string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=5", remote,
			fmt.Sprintf("tmux new-window -t %s:", sessionName))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return sessionsMsg{err: fmt.Errorf("failed to create window: %w: %s", err, string(out))}
		}
		return statusMsg(fmt.Sprintf("Created new window in '%s'", sessionName))
	}
}

func autoRefreshTick() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func openSessionTab(remote, sessionName, terminal string, useSSH bool) tea.Cmd {
	return func() tea.Msg {
		var connectCmd string
		if useSSH {
			connectCmd = fmt.Sprintf(" ssh -t %s 'tmux new -A -s %s'", remote, sessionName)
		} else {
			connectCmd = fmt.Sprintf(" mosh %s -- tmux new -A -s %s", remote, sessionName)
		}

		var script string
		switch terminal {
		case "iterm2":
			script = fmt.Sprintf(`tell application "iTerm2"
	tell current window
		set newTab to (create tab with default profile)
		tell current session of newTab
			set name to "%s"
			write text "%s"
		end tell
	end tell
end tell`, sessionName, connectCmd)
		case "terminal":
			script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
	set custom title of newTab to "%s"
end tell`, connectCmd, sessionName)
		default:
			// Fallback: just run in background
			script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
end tell`, connectCmd)
		}

		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run() // Wait for tab to be created

		return tabOpenedMsg{name: sessionName}
	}
}

func openSessionWindowTab(remote, sessionName string, windowIndex int, terminal string, useSSH bool) tea.Cmd {
	return func() tea.Msg {
		var connectCmd string
		if useSSH {
			connectCmd = fmt.Sprintf(` ssh -t %s 'tmux new -A -s %s \\; select-window -t %s:%d'`, remote, sessionName, sessionName, windowIndex)
		} else {
			connectCmd = fmt.Sprintf(` mosh %s -- tmux new -A -s %s \\; select-window -t %s:%d`, remote, sessionName, sessionName, windowIndex)
		}

		var script string
		switch terminal {
		case "iterm2":
			tabName := fmt.Sprintf("%s:%d", sessionName, windowIndex)
			script = fmt.Sprintf(`tell application "iTerm2"
	tell current window
		set newTab to (create tab with default profile)
		tell current session of newTab
			set name to "%s"
			write text "%s"
		end tell
	end tell
end tell`, tabName, connectCmd)
		case "terminal":
			tabName := fmt.Sprintf("%s:%d", sessionName, windowIndex)
			script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
	set custom title of newTab to "%s"
end tell`, connectCmd, tabName)
		default:
			script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
end tell`, connectCmd)
		}

		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run()

		return tabOpenedMsg{name: fmt.Sprintf("%s:%d", sessionName, windowIndex)}
	}
}

func openAllSessionTabs(remote string, sessions []tmuxSession, terminal string, useSSH bool) tea.Cmd {
	return func() tea.Msg {
		for _, s := range sessions {
			var connectCmd string
			if useSSH {
				connectCmd = fmt.Sprintf(" ssh -t %s 'tmux new -A -s %s'", remote, s.Name)
			} else {
				connectCmd = fmt.Sprintf(" mosh %s -- tmux new -A -s %s", remote, s.Name)
			}

			var script string
			switch terminal {
			case "iterm2":
				script = fmt.Sprintf(`tell application "iTerm2"
	tell current window
		set newTab to (create tab with default profile)
		tell current session of newTab
			set name to "%s"
			write text "%s"
		end tell
	end tell
end tell`, s.Name, connectCmd)
			case "terminal":
				script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
	set custom title of newTab to "%s"
end tell`, connectCmd, s.Name)
			default:
				script = fmt.Sprintf(`tell application "Terminal"
	activate
	set newTab to do script "%s"
end tell`, connectCmd)
			}

			cmd := exec.Command("osascript", "-e", script)
			_ = cmd.Start()
			time.Sleep(300 * time.Millisecond) // small delay between tabs
		}
		return statusMsg(fmt.Sprintf("Opened %d sessions", len(sessions)))
	}
}

func closeRemoteTabs(remote, terminal string) tea.Cmd {
	return func() tea.Msg {
		var script string
		switch terminal {
		case "iterm2":
			script = fmt.Sprintf(`tell application "iTerm2"
	tell current window
		set tabCount to count of tabs
		repeat with i from tabCount to 1 by -1
			set t to item i of tabs
			tell current session of t
				set tabName to name
			end tell
			if tabName contains "%s" then
				close t
			end if
		end repeat
	end tell
end tell`, remote)
		case "terminal":
			script = fmt.Sprintf(`tell application "Terminal"
	set windowCount to count of windows
	repeat with w in windows
		set tabCount to count of tabs of w
		repeat with i from tabCount to 1 by -1
			set t to item i of tabs of w
			if custom title of t contains "%s" then
				close t
			end if
		end repeat
	end repeat
end tell`, remote)
		default:
			return statusMsg("Tab closing not supported for this terminal")
		}

		cmd := exec.Command("osascript", "-e", script)
		_ = cmd.Run()
		return statusMsg("Closed remote tabs")
	}
}

func killSession(remote, name string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=5", remote,
			fmt.Sprintf("tmux kill-session -t '%s'", name))
		if err := cmd.Run(); err != nil {
			return sessionsMsg{err: fmt.Errorf("failed to kill session: %w", err)}
		}
		return statusMsg(fmt.Sprintf("Killed session '%s'", name))
	}
}

// ---------------------------------------------------------------------------
// Init / Update / View
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchSessions(m.remote),
		autoRefreshTick(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsMsg:
		m.loading = false
		if msg.err != nil {
			m.status = msg.err.Error()
			m.statusErr = true
			return m, clearStatusAfter(5 * time.Second)
		}
		m.sessions = msg.sessions
		// Clear "Refreshing..." status
		if m.status == "Refreshing..." {
			m.status = ""
		}
		if m.cursor >= m.totalItems() {
			m.cursor = max(0, m.totalItems()-1)
		}
		return m, nil

	case tickMsg:
		// Auto-refresh
		return m, tea.Batch(fetchSessions(m.remote), autoRefreshTick())

	case statusMsg:
		m.status = string(msg)
		m.statusErr = false
		// Also refresh sessions
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			fetchSessions(m.remote),
			clearStatusAfter(3*time.Second),
		)

	case clearStatusMsg:
		m.status = ""
		m.statusErr = false
		return m, nil

	case tabOpenedMsg:
		m.status = fmt.Sprintf("Opened '%s' in new tab", msg.name)
		m.statusErr = false
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			fetchSessions(m.remote),
			clearStatusAfter(3*time.Second),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Handle based on current mode
		switch m.mode {

		case viewNewSession:
			switch msg.String() {
			case "esc":
				m.mode = viewNormal
				m.textInput.Reset()
				return m, nil
			case "enter":
				name := strings.TrimSpace(m.textInput.Value())
				if name != "" {
					m.mode = viewNormal
					m.textInput.Reset()
					return m, openSessionTab(m.remote, name, m.terminal, m.useSSH)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}

		case viewConfirmKill:
			switch msg.String() {
			case "y", "enter":
				vis := m.visibleItems()
				if m.cursor >= 0 && m.cursor < len(vis) {
					item := vis[m.cursor]
					m.mode = viewNormal
					m.loading = true
					if item.kind == "window" {
						s := m.sessions[item.sessionIdx]
						w := s.WinList[item.windowIdx]
						return m, tea.Batch(m.spinner.Tick, killWindow(m.remote, s.Name, w.Index))
					} else if item.kind == "session" {
						return m, tea.Batch(m.spinner.Tick, killSession(m.remote, m.sessions[item.sessionIdx].Name))
					}
				}
				m.mode = viewNormal
				return m, nil
			case "n", "esc", "q":
				m.mode = viewNormal
				return m, nil
			}
			return m, nil

		case viewQuit:
			switch msg.String() {
			case "q":
				// q->q: close all tabs & quit
				return m, tea.Sequence(
					closeRemoteTabs(m.remote, m.terminal),
					func() tea.Msg { return nil },
					tea.Quit,
				)
			case "esc":
				m.mode = viewNormal
				return m, nil
			case "up", "k":
				if m.quitIdx > 0 {
					m.quitIdx--
				}
				return m, nil
			case "down", "j":
				if m.quitIdx < 2 {
					m.quitIdx++
				}
				return m, nil
			case "enter":
				switch m.quitIdx {
				case 0: // Close tabs & quit
					return m, tea.Sequence(
						closeRemoteTabs(m.remote, m.terminal),
						func() tea.Msg { return nil },
						tea.Quit,
					)
				case 1: // Just quit
					return m, tea.Quit
				case 2: // Cancel
					m.mode = viewNormal
					return m, nil
				}
			}
			return m, nil

		case viewNormal:
			total := m.totalItems()
			switch msg.String() {
			case "ctrl+l", "ctrl+r":
				// Ignore — prevents accidental screen clear (Cmd+R in iTerm2)
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				} else {
					m.cursor = total - 1
				}
				// Skip disabled kill item
				if !m.isKillEnabled() && m.isOnKillItem() {
					if m.cursor > 0 {
						m.cursor--
					} else {
						m.cursor = total - 1
					}
				}
				return m, nil
			case "down", "j":
				if m.cursor < total-1 {
					m.cursor++
				} else {
					m.cursor = 0
				}
				// Skip disabled kill item
				if !m.isKillEnabled() && m.isOnKillItem() {
					if m.cursor < total-1 {
						m.cursor++
					} else {
						m.cursor = 0
					}
				}
				return m, nil
			case "right", "l":
				// Expand selected session
				items := m.visibleItems()
				if m.cursor >= 0 && m.cursor < len(items) {
					item := items[m.cursor]
					if item.kind == "session" {
						sessName := m.sessions[item.sessionIdx].Name
						if !m.expanded[sessName] {
							m.expanded[sessName] = true
						}
					}
				}
				return m, nil
			case "left", "h":
				// Collapse selected session (or parent session if on a window)
				items := m.visibleItems()
				if m.cursor >= 0 && m.cursor < len(items) {
					item := items[m.cursor]
					if item.kind == "session" {
						sessName := m.sessions[item.sessionIdx].Name
						m.expanded[sessName] = false
					} else if item.kind == "window" {
						// Collapse the parent session and move cursor to it
						sessName := m.sessions[item.sessionIdx].Name
						m.expanded[sessName] = false
						// Find the session row in visible items and move cursor there
						newItems := m.visibleItems()
						for ni, nItem := range newItems {
							if nItem.kind == "session" && nItem.sessionIdx == item.sessionIdx {
								m.cursor = ni
								break
							}
						}
					}
				}
				return m, nil
			case "enter":
				return m.handleEnter()
			case "a":
				if len(m.sessions) > 0 {
					return m, openAllSessionTabs(m.remote, m.sessions, m.terminal, m.useSSH)
				}
				return m, nil
			case "w":
				// Create new window in the selected session
				sessIdx := m.selectedSessionIdx()
				if sessIdx >= 0 && sessIdx < len(m.sessions) {
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, createWindow(m.remote, m.sessions[sessIdx].Name))
				}
				m.status = "Select a session first"
				m.statusErr = true
				return m, clearStatusAfter(2 * time.Second)
			case "n":
				m.mode = viewNewSession
				m.textInput.Reset()
				m.textInput.Focus()
				return m, m.textInput.Cursor.BlinkCmd()
			case "r":
				m.loading = true
				m.status = "Refreshing..."
				m.statusErr = false
				return m, tea.Batch(m.spinner.Tick, fetchSessions(m.remote))
			case "x", "delete", "backspace":
				if m.isKillEnabled() {
					m.mode = viewConfirmKill
					return m, nil
				}
				return m, nil
			case "q":
				m.mode = viewQuit
				m.quitIdx = 0
				return m, nil
			default:
				// Number keys 1-9
				if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
					idx := int(msg.String()[0] - '1')
					if idx < len(m.sessions) {
						s := m.sessions[idx]
						return m, openSessionTab(m.remote, s.Name, m.terminal, m.useSSH)
					}
				}
			}
		}
	}

	// Update text input if in new session mode
	if m.mode == viewNewSession {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return m, nil
	}
	item := items[m.cursor]

	switch item.kind {
	case "session":
		if item.sessionIdx >= 0 && item.sessionIdx < len(m.sessions) {
			s := m.sessions[item.sessionIdx]
			return m, openSessionTab(m.remote, s.Name, m.terminal, m.useSSH)
		}
	case "window":
		if item.sessionIdx >= 0 && item.sessionIdx < len(m.sessions) {
			s := m.sessions[item.sessionIdx]
			if item.windowIdx >= 0 && item.windowIdx < len(s.WinList) {
				w := s.WinList[item.windowIdx]
				return m, openSessionWindowTab(m.remote, s.Name, w.Index, m.terminal, m.useSSH)
			}
		}
	case "menu":
		menuIdx := item.menuIdx
		menus := m.menuItems()
		if menuIdx < 0 || menuIdx >= len(menus) {
			return m, nil
		}
		switch menus[menuIdx].id {
		case menuOpenAll:
			return m, openAllSessionTabs(m.remote, m.sessions, m.terminal, m.useSSH)
		case menuKill:
			m.mode = viewConfirmKill
			return m, nil
		case menuNewWindow:
			sessIdx := m.selectedSessionIdx()
			if sessIdx >= 0 && sessIdx < len(m.sessions) {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, createWindow(m.remote, m.sessions[sessIdx].Name))
			}
			return m, nil
		case menuNew:
			m.mode = viewNewSession
			m.textInput.Reset()
			m.textInput.Focus()
			return m, m.textInput.Cursor.BlinkCmd()
		case menuRefresh:
			m.loading = true
			m.status = "Refreshing..."
			m.statusErr = false
			return m, tea.Batch(m.spinner.Tick, fetchSessions(m.remote))
		case menuQuit:
			m.mode = viewQuit
			m.quitIdx = 0
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	lineWidth := max(10, m.width-4)

	// ── Header ──
	transport := "mosh"
	if m.useSSH {
		transport = "ssh"
	}
	termLabel := m.terminal
	if termLabel == "iterm2" {
		termLabel = "iTerm2"
	} else if termLabel == "terminal" {
		termLabel = "Terminal.app"
	}

	// Header — ANSI Shadow figlet logo, single line with gradient
	logo := []string{
		"████████╗███╗   ███╗██╗   ██╗██╗  ██╗     ██████╗ ██████╗ ███╗   ██╗███╗   ██╗███████╗ ██████╗████████╗",
		"╚══██╔══╝████╗ ████║██║   ██║╚██╗██╔╝    ██╔════╝██╔═══██╗████╗  ██║████╗  ██║██╔════╝██╔════╝╚══██╔══╝",
		"   ██║   ██╔████╔██║██║   ██║ ╚███╔╝     ██║     ██║   ██║██╔██╗ ██║██╔██╗ ██║█████╗  ██║        ██║   ",
		"   ██║   ██║╚██╔╝██║██║   ██║ ██╔██╗     ██║     ██║   ██║██║╚██╗██║██║╚██╗██║██╔══╝  ██║        ██║   ",
		"   ██║   ██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗    ╚██████╗╚██████╔╝██║ ╚████║██║ ╚████║███████╗╚██████╗   ██║   ",
		"   ╚═╝   ╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝     ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═══╝╚══════╝ ╚═════╝   ╚═╝   ",
	}
	logoColors := []lipgloss.Color{"#00E5FF", "#00C0FF", "#8050FF", "#BD00FF", "#E000D0", "#FF2D95"}

	b.WriteString("\n")
	for i, line := range logo {
		b.WriteString(lipgloss.NewStyle().Foreground(logoColors[i]).Bold(true).Render(line) + "\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render("  ── remote session manager ──") + "\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#4A5568")).Render("  "+m.quote) + "\n")
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorSecondary).Render("  " + strings.Repeat("─", lineWidth)) + "\n")
	b.WriteString("\n")

	// Host info bar
	hostLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(colorDim).Render("  \u25C8 "),
		titleStyle.Render(m.remote),
		lipgloss.NewStyle().Foreground(colorDim).Render("  \u2502 "),
		lipgloss.NewStyle().Foreground(colorMagenta).Render(transport),
		lipgloss.NewStyle().Foreground(colorDim).Render("  \u2502 "),
		lipgloss.NewStyle().Foreground(colorElecBlue).Render(termLabel),
		lipgloss.NewStyle().Foreground(colorDim).Render("  \u25C8"),
	)
	b.WriteString(hostLine + "\n")

	// Thin separator
	thinSep := lipgloss.NewStyle().Foreground(colorDim).Render(
		"  " + strings.Repeat("\u2508", max(5, lineWidth-4)))
	b.WriteString(thinSep + "\n\n")

	// ── Loading state ──
	if m.loading && len(m.sessions) == 0 {
		loadBox := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorMagenta).
			Padding(1, 3).
			Render(
				m.spinner.View() + " " +
					lipgloss.NewStyle().Foreground(colorPrimary).Render("\u2588\u2593\u2592\u2591 ") +
					lipgloss.NewStyle().Foreground(colorMuted).Render("Establishing link to ") +
					lipgloss.NewStyle().Foreground(colorPrimary).Render(m.remote) +
					lipgloss.NewStyle().Foreground(colorPrimary).Render(" \u2591\u2592\u2593\u2588"))
		b.WriteString("\n" + loadBox + "\n")
		return b.String()
	}

	boxWidth := max(10, m.width-6)

	// Build visible items for cursor tracking
	vis := m.visibleItems()

	// ── Session list ──
	if len(m.sessions) == 0 && !m.loading {
		emptyMsg := lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(1, 2).
			Render("\u25B3 No active sessions detected. Press " +
				lipgloss.NewStyle().Foreground(colorYellow).Render("[n]") +
				lipgloss.NewStyle().Foreground(colorDim).Render(" to initialize one."))
		b.WriteString(sessionBoxStyle.Width(boxWidth).Render(emptyMsg) + "\n")
	} else {
		var boxItems []string
		loadingIndicator := ""
		if m.loading {
			loadingIndicator = "  " + m.spinner.View()
		}

		sectionHeader := lipgloss.NewStyle().Foreground(colorMagenta).Render("\u2590") +
			lipgloss.NewStyle().Foreground(colorPrimary).
				Render(fmt.Sprintf(" ACTIVE SESSIONS \u00AB%d\u00BB", len(m.sessions))) +
			loadingIndicator
		boxItems = append(boxItems, sectionHeader)

		// Thin inner separator
		innerSep := lipgloss.NewStyle().Foreground(colorDim).Render(
			strings.Repeat("\u2500", max(5, boxWidth-6)))
		boxItems = append(boxItems, innerSep)

		// Render each visible item (sessions and windows)
		for visIdx, vi := range vis {
			if vi.kind == "menu" {
				break // menu items are rendered in the bottom bar
			}

			if vi.kind == "session" {
				i := vi.sessionIdx
				s := m.sessions[i]

				shortcut := "   "
				if i < 9 {
					shortcut = shortcutStyle.Render(fmt.Sprintf("[%d]", i+1))
				}

				isSelected := visIdx == m.cursor && (m.mode == viewNormal || m.mode == viewConfirmKill)
				cursor := "  "
				style := sessionItemStyle
				if isSelected {
					cursor = cursorStyle.Render("\u25B8 ")
					style = sessionSelectedStyle
				}

				dot := detachedDot
				if s.Attached {
					dot = attachedDot
				}

				name := sessionNameStyle.Render(s.Name)
				info := sessionInfoStyle.Render(fmt.Sprintf("  %d window%s", s.Windows, pluralize(s.Windows)))

				// Activity indicator
				activity := ""
				if s.Activity != "" && s.Activity != "zsh" && s.Activity != "bash" && s.Activity != "fish" {
					activity = lipgloss.NewStyle().Foreground(colorYellow).Render(fmt.Sprintf("  \u2302 %s", s.Activity))
				}

				// Expand/collapse indicator
				expandIndicator := ""
				if m.expanded[s.Name] {
					expandIndicator = lipgloss.NewStyle().Foreground(colorDim).Render("  \u25BE") // ▾
				} else if s.Windows > 1 {
					expandIndicator = lipgloss.NewStyle().Foreground(colorDim).Render("  \u25B8") // ▸
				}

				prefix := lipgloss.NewStyle().Foreground(colorDim).Render("\u2502")
				line := prefix + " " + style.Render(fmt.Sprintf("%s %s %s %s%s%s%s", shortcut, cursor, dot, name, info, activity, expandIndicator))
				boxItems = append(boxItems, line)
			} else if vi.kind == "window" {
				s := m.sessions[vi.sessionIdx]
				w := s.WinList[vi.windowIdx]
				isLast := vi.windowIdx == len(s.WinList)-1

				isSelected := visIdx == m.cursor && (m.mode == viewNormal || m.mode == viewConfirmKill)

				// Tree drawing characters
				treeChar := "\u251C\u2500" // ├─
				if isLast {
					treeChar = "\u2514\u2500" // └─
				}

				// Build window line content
				activeMarker := ""
				if w.Active {
					activeMarker = "*"
				}

				cmdInfo := ""
				if w.Command != "" && w.Command != "zsh" && w.Command != "bash" && w.Command != "fish" {
					cmdInfo = lipgloss.NewStyle().Foreground(colorYellow).Render(fmt.Sprintf("  \u26A1%s", w.Command))
				}

				winName := fmt.Sprintf("%d: %s%s", w.Index, w.Name, activeMarker)

				style := windowItemStyle
				if isSelected {
					style = windowSelectedStyle
				}

				prefix := lipgloss.NewStyle().Foreground(colorDim).Render("\u2502")
				treeStyle := lipgloss.NewStyle().Foreground(colorDim)
				line := prefix + "         " + treeStyle.Render(treeChar) + " " + style.Render(winName+cmdInfo)
				boxItems = append(boxItems, line)
			}
		}

		// Legend
		boxItems = append(boxItems, lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("\u2500", max(5, boxWidth-6))))
		legend := lipgloss.NewStyle().Foreground(colorDim).Render(
			fmt.Sprintf("  %s attached  %s idle  \u2302 process", attachedDot, detachedDot))
		boxItems = append(boxItems, legend)

		box := sessionBoxStyle.Width(boxWidth).Render(strings.Join(boxItems, "\n"))
		b.WriteString(box + "\n")
	}

	// ── Confirm kill ──
	if m.mode == viewConfirmKill {
		vis := m.visibleItems()
		if m.cursor >= 0 && m.cursor < len(vis) {
			item := vis[m.cursor]
			var headerText, targetText string
			if item.kind == "window" && item.sessionIdx < len(m.sessions) {
				s := m.sessions[item.sessionIdx]
				if item.windowIdx < len(s.WinList) {
					w := s.WinList[item.windowIdx]
					headerText = "  \u26A0 TERMINATE WINDOW"
					targetText = fmt.Sprintf("  \u2192 %s:%d (%s)", s.Name, w.Index, w.Name)
				}
			} else if item.kind == "session" && item.sessionIdx < len(m.sessions) {
				headerText = "  \u26A0 TERMINATE SESSION"
				targetText = fmt.Sprintf("  \u2192 '%s'", m.sessions[item.sessionIdx].Name)
			}
			if headerText != "" {
				confirmHeader := lipgloss.NewStyle().Foreground(colorRed).Render(headerText)
				confirmMsg := lipgloss.NewStyle().Foreground(colorHotPink).Render(targetText)
				confirmHint := lipgloss.NewStyle().Foreground(colorDim).
					Render("  " + lipgloss.NewStyle().Foreground(colorAccent).Render("[y]") + " confirm  " +
						lipgloss.NewStyle().Foreground(colorRed).Render("[n]") + " abort")
				confirmContent := confirmHeader + "\n" + confirmMsg + "\n\n" + confirmHint
				killBox := lipgloss.NewStyle().
					Border(lipgloss.DoubleBorder()).
					BorderForeground(colorRed).
					Padding(0, 1).
					MarginTop(1).
					Width(min(55, m.width-6)).
					Render(confirmContent)
				b.WriteString(killBox + "\n")
			}
		}
	}

	// ── New session input (inline, right after sessions) ──
	if m.mode == viewNewSession {
		inputHeader := lipgloss.NewStyle().Foreground(colorMagenta).
			Render("  \u2590 NEW SESSION") + "\n" +
			lipgloss.NewStyle().Foreground(colorDim).Render("  "+strings.Repeat("\u2500", 20))
		inputContent := inputHeader + "\n\n" +
			"  " + m.textInput.View() + "\n\n" +
			lipgloss.NewStyle().Foreground(colorDim).Render("  ") +
			lipgloss.NewStyle().Foreground(colorAccent).Render("enter") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" confirm  \u2502  ") +
			lipgloss.NewStyle().Foreground(colorRed).Render("esc") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" cancel")

		b.WriteString(inputBoxStyle.Width(min(55, m.width-6)).Render(inputContent) + "\n")
	}

	// ── Quit menu (inline) ──
	if m.mode == viewQuit {
		quitTitle := lipgloss.NewStyle().Foreground(colorRed).
			Render("  \u2590 EXIT PROTOCOL")

		opts := []string{
			"Close all tabs & quit",
			"Just quit",
			"Cancel",
		}
		optIcons := []string{"\u2612", "\u2190", "\u21BA"}
		var quitItems []string
		quitItems = append(quitItems, quitTitle)
		quitItems = append(quitItems, lipgloss.NewStyle().Foreground(colorDim).Render("  "+strings.Repeat("\u2500", 25)))
		for i, opt := range opts {
			cursor := "  "
			style := quitOptionStyle
			if i == m.quitIdx {
				cursor = cursorStyle.Render("\u25B8 ")
				style = quitSelectedStyle
			}
			icon := lipgloss.NewStyle().Foreground(colorDim).Render(optIcons[i])
			quitItems = append(quitItems, fmt.Sprintf("  %s%s %s", cursor, icon, style.Render(opt)))
		}
		quitItems = append(quitItems, "")
		quitItems = append(quitItems, lipgloss.NewStyle().Foreground(colorDim).Render("  ")+
			lipgloss.NewStyle().Foreground(colorAccent).Render("enter")+
			lipgloss.NewStyle().Foreground(colorDim).Render(" select  \u2502  ")+
			lipgloss.NewStyle().Foreground(colorPrimary).Render("q")+
			lipgloss.NewStyle().Foreground(colorDim).Render(" close all & quit  \u2502  ")+
			lipgloss.NewStyle().Foreground(colorRed).Render("esc")+
			lipgloss.NewStyle().Foreground(colorDim).Render(" back"))

		b.WriteString(quitBoxStyle.Width(min(55, m.width-6)).Render(strings.Join(quitItems, "\n")) + "\n")
	}

	// ── Status ──
	if m.status != "" {
		if m.statusErr {
			b.WriteString(errorStyle.Render("  \u2718 "+m.status) + "\n")
		} else {
			b.WriteString(statusStyle.Render("  \u2714 "+m.status) + "\n")
		}
	}

	topContent := b.String()
	topHeight := strings.Count(topContent, "\n")

	// ── Bottom bar: horizontal menu ──
	var bottom strings.Builder
	if m.mode == viewNormal {
		killEnabled := m.isKillEnabled()
		var menuParts []string
		menus := m.menuItems()
		for mi, item := range menus {
			if (item.id == menuKill || item.id == menuNewWindow) && !killEnabled {
				continue
			}

			// Find this menu item's index in visible items
			globalIdx := -1
			for vi, v := range vis {
				if v.kind == "menu" && v.menuIdx == mi {
					globalIdx = vi
					break
				}
			}

			isSelected := globalIdx == m.cursor
			var part string
			keyStyle := lipgloss.NewStyle().Foreground(colorDim)
			labelStyle := lipgloss.NewStyle().Foreground(colorMuted)

			if item.id == menuKill {
				keyStyle = lipgloss.NewStyle().Foreground(colorRed)
				labelStyle = lipgloss.NewStyle().Foreground(colorRed)
			}
			if isSelected {
				keyStyle = lipgloss.NewStyle().Foreground(colorPrimary)
				labelStyle = lipgloss.NewStyle().Foreground(colorWhite)
				if item.id == menuKill {
					keyStyle = lipgloss.NewStyle().Foreground(colorRed)
					labelStyle = lipgloss.NewStyle().Foreground(colorRed)
				}
				part = cursorStyle.Render("\u25B8") + keyStyle.Render(item.key) + " " + labelStyle.Render(item.label)
			} else {
				part = keyStyle.Render(item.key) + " " + labelStyle.Render(item.label)
			}

			menuParts = append(menuParts, part)
		}

		sep := lipgloss.NewStyle().Foreground(colorDim).Render("  \u2502  ")
		barLine := "  " + strings.Join(menuParts, sep)

		bottom.WriteString(lipgloss.NewStyle().Foreground(colorSecondary).Render("  "+strings.Repeat("\u2500", lineWidth)) + "\n")
		bottom.WriteString(barLine + "\n")

		// Hint line
		hintLine := lipgloss.NewStyle().Foreground(colorDim).Render("  ") +
			lipgloss.NewStyle().Foreground(colorCyan).Render("\u2191\u2193") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" navigate  ") +
			lipgloss.NewStyle().Foreground(colorCyan).Render("\u2190\u2192") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" expand  ") +
			lipgloss.NewStyle().Foreground(colorCyan).Render("enter") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" open  ") +
			lipgloss.NewStyle().Foreground(colorCyan).Render("1-9") +
			lipgloss.NewStyle().Foreground(colorDim).Render(" quick open")
		bottom.WriteString(hintLine + "\n")
	}
	bottomContent := bottom.String()
	bottomHeight := strings.Count(bottomContent, "\n")

	// Fill gap between top and bottom
	gap := m.height - topHeight - bottomHeight - 1
	if gap < 1 {
		gap = 1
	}

	return topContent + strings.Repeat("\n", gap) + bottomContent
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	useSSH := false
	args := os.Args[1:]

	// Parse flags
	var remote string
	for _, arg := range args {
		switch arg {
		case "--ssh":
			useSSH = true
		case "-h", "--help":
			fmt.Println(helpText())
			os.Exit(0)
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
				os.Exit(1)
			}
			remote = arg
		}
	}

	if remote == "" {
		fmt.Fprintln(os.Stderr, "Usage: tmux-connect [--ssh] <remote>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run with --help for more information.")
		os.Exit(1)
	}

	p := tea.NewProgram(
		initialModel(remote, useSSH),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	// Clear scrollback so the alt screen content doesn't linger
	fmt.Print("\033[2J\033[H")
}

func helpText() string {
	return `tmux-connect - tmux session manager for remote servers

Usage:
  tmux-connect [--ssh] <remote>

Options:
  --ssh    Use ssh instead of mosh (default: mosh)
  -h       Show this help

Controls:
  Up/Down    Navigate sessions and windows
  Left/Right Collapse/expand session windows
  Enter      Open session in new tab
  1-9        Open session by number
  a          Open all sessions
  n          Create new session
  r          Refresh session list
  q          Quit menu

Auto-refreshes every 10 seconds. Commands are prefixed
with a space to avoid polluting shell history.`
}
