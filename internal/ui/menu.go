// Package ui: menu.go builds the menu bar and the Help and About windows.
//
// NATIVE MENUS
// Fyne turns the fyne.MainMenu given to SetMainMenu into the platform's own
// menu: the real menu bar at the top of the screen on macOS, and a menu bar
// inside the window on Windows. A few conventions make it feel native:
//   - On macOS a menu item labelled exactly "About" is moved by Fyne into the
//     application menu, where macOS users expect it, so the label must be
//     "About" there. On Windows the Help menu says "About My Capture".
//   - Fyne adds the Quit item itself (in the application menu on macOS, at
//     the end of the first menu elsewhere), so none is created here.
//   - Menu shortcuts use fyne.KeyModifierShortcutDefault, which is Cmd on
//     macOS and Ctrl on Windows. Fyne only fires a menu shortcut when a
//     modifier is held, so the plain keys (F, F11, A, C, F1, Escape) are handled
//     by the key handler in app.go and are listed in the help, not here.
//
// HELP
// The help is help.md, compiled into the program, so it is always available
// and matches the version being run. The environment variable section is
// added from internal/config, the same text that -help prints.
package ui

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"my-capture/internal/config"
	"my-capture/internal/video"
)

// -----------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------

const (
	// repoURL is the project's home page on GitHub, opened by the Help menu.
	repoURL = "https://github.com/steveohara/my-capture"

	// issuesURL is where bugs are reported.
	issuesURL = repoURL + "/issues"
)

// helpMarkdown is the text of the help window, from help.md.
//
//go:embed help.md
var helpMarkdown string

// -----------------------------------------------------------------------
// Menu bar
// -----------------------------------------------------------------------

// buildMenus creates the menu bar and installs it in the window.
func (a *App) buildMenus() {
	shortcut := func(key fyne.KeyName) fyne.Shortcut {
		return &desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierShortcutDefault}
	}

	refresh := fyne.NewMenuItem("Refresh Devices", a.refreshDevices)
	refresh.Shortcut = shortcut(fyne.KeyR)
	file := fyne.NewMenu("File", refresh)

	fullscreen := fyne.NewMenuItem("Full Screen", a.toggleFullscreen)
	fullscreen.Shortcut = shortcut(fyne.KeyF)
	a.controlsItem = fyne.NewMenuItem("Hide Controls", a.toggleControls)
	a.controlsItem.Shortcut = &desktop.CustomShortcut{
		KeyName: fyne.KeyC, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}
	nextAspect := fyne.NewMenuItem("Next Aspect", a.cycleAspect)

	aspect := fyne.NewMenu("Aspect")
	for i, m := range AspectModes {
		item := fyne.NewMenuItem(m.Name, func() { a.aspectSel.SetSelectedIndex(i) })
		item.Checked = i == a.aspectSel.SelectedIndex()
		a.aspectItems = append(a.aspectItems, item)
		aspect.Items = append(aspect.Items, item)
	}
	aspectItem := fyne.NewMenuItem("Aspect", nil)
	aspectItem.ChildMenu = aspect
	view := fyne.NewMenu("View", fullscreen, a.controlsItem, fyne.NewMenuItemSeparator(), aspectItem, nextAspect)

	aboutLabel := "About My Capture"
	if runtime.GOOS == "darwin" {
		aboutLabel = "About" // the exact word Fyne looks for, see the package comment
	}
	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("My Capture Help", a.showHelp),
		fyne.NewMenuItem("My Capture on GitHub", func() { a.openURL(repoURL) }),
		fyne.NewMenuItem("Report an Issue...", func() { a.openURL(issuesURL) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(aboutLabel, a.showAbout))

	a.mainMenu = fyne.NewMainMenu(file, view, help)
	a.win.SetMainMenu(a.mainMenu)
}

// syncAspectMenu puts the check mark in the Aspect submenu on the mode that
// is selected in the picker, so the menu and the picker always agree.
func (a *App) syncAspectMenu() {
	if a.mainMenu == nil {
		return // the menu bar is not built yet
	}
	selected := a.aspectSel.SelectedIndex()
	for i, item := range a.aspectItems {
		item.Checked = i == selected
	}
	a.mainMenu.Refresh()
}

// syncControlsMenu sets the label of the View menu entry to the action it
// would do now, and greys it out in full screen, where the controls cannot be
// brought back.
func (a *App) syncControlsMenu() {
	if a.controlsItem == nil || a.mainMenu == nil {
		return // the menu bar is not built yet
	}
	a.controlsItem.Label = "Hide Controls"
	if a.controlsHidden {
		a.controlsItem.Label = "Show Controls"
	}
	a.controlsItem.Disabled = a.fullscreen
	a.mainMenu.Refresh()
}

// openURL opens a web page in the user's browser. A failure is reported in
// the status line, because the Help menu has nowhere better to show it.
func (a *App) openURL(address string) {
	u, err := url.Parse(address)
	if err != nil {
		a.setStatus("Could not open " + address + ": " + err.Error())
		return
	}
	if err := a.fyneApp.OpenURL(u); err != nil {
		a.setStatus("Could not open the browser: " + err.Error())
	}
}

// -----------------------------------------------------------------------
// Help window
// -----------------------------------------------------------------------

// showHelp opens the help window, or brings it to the front if it is open.
func (a *App) showHelp() {
	if a.helpWin != nil {
		a.helpWin.RequestFocus()
		return
	}

	// The environment variable text is the same as the -help output, shown
	// as a code block to keep its layout.
	text := helpMarkdown + "\n## Environment variables\n\n```\n" + config.Usage() + "```\n"

	// Word wrapping, and a scroll that only goes up and down, so the text
	// wraps at the window's edge instead of running off to the right.
	rich := widget.NewRichTextFromMarkdown(text)
	rich.Wrapping = fyne.TextWrapWord

	w := a.fyneApp.NewWindow("My Capture Help")
	w.SetContent(container.NewVScroll(rich))
	w.Resize(fyne.NewSize(760, 640))
	w.SetOnClosed(func() { a.helpWin = nil })
	a.helpWin = w
	w.Show()
}

// -----------------------------------------------------------------------
// About
// -----------------------------------------------------------------------

// showAbout opens the About box: name, version, what the program is built
// with, a link to the project, and details useful in a bug report.
func (a *App) showAbout() {
	center := fyne.TextAlignCenter

	title := widget.NewLabelWithStyle("My Capture", center, fyne.TextStyle{Bold: true})
	version := widget.NewLabelWithStyle("Version "+a.version, center, fyne.TextStyle{})
	about := widget.NewLabelWithStyle(
		"Shows the picture and plays the sound of a USB video capture device.",
		center, fyne.TextStyle{})
	about.Wrapping = fyne.TextWrapWord
	credits := widget.NewLabelWithStyle(
		"Built with Fyne, miniaudio and ffmpeg.", center, fyne.TextStyle{})

	var link fyne.CanvasObject = widget.NewLabel(repoURL)
	if u, err := url.Parse(repoURL); err == nil {
		h := widget.NewHyperlink("github.com/steveohara/my-capture", u)
		h.Alignment = center
		link = h
	}

	details := widget.NewLabel(a.aboutDetails("checking..."))
	details.TextStyle = fyne.TextStyle{Monospace: true}
	details.Wrapping = fyne.TextWrapWord

	copyBtn := widget.NewButton("Copy details", func() {
		a.fyneApp.Clipboard().SetContent(details.Text)
	})

	content := container.NewVBox(title, version, about, link, credits,
		widget.NewSeparator(), details, copyBtn)
	d := dialog.NewCustom("About My Capture", "Close", content, a.win)
	d.Resize(fyne.NewSize(480, 0))
	d.Show()

	// Asking ffmpeg for its version takes a moment, so it fills in later.
	if a.ffmpegErr != nil {
		details.SetText(a.aboutDetails("not found"))
		return
	}
	ffmpeg := a.ffmpeg
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		v, err := video.FFmpegVersion(ctx, ffmpeg)
		if err != nil || v == "" {
			v = "version unknown"
		}
		a.ui(func() { details.SetText(a.aboutDetails(v)) })
	}()
}

// aboutDetails returns the plain text block of the About box, which the Copy
// details button puts on the clipboard. ffmpegVersion is shown on the line
// below the ffmpeg path.
func (a *App) aboutDetails(ffmpegVersion string) string {
	ffmpegPath := a.ffmpeg
	if a.ffmpegErr != nil {
		ffmpegPath = "not found"
	}
	return fmt.Sprintf("my-capture %s\n%s/%s, %s\nffmpeg: %s\n        %s",
		a.version, runtime.GOOS, runtime.GOARCH, runtime.Version(),
		ffmpegPath, ffmpegVersion)
}
