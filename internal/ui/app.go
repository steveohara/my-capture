// Package ui: app.go builds the my-capture window and connects the device
// pickers to the video and audio back ends.
//
// WHAT THE USER SEES
// A strip of controls above the picture: the video device, its mode
// (resolution and frame rate), the audio input and the audio output. Choosing
// a video device starts showing it at once, and choosing an audio input or
// output starts or moves the sound at once, so there are no Start or Stop
// buttons. Choosing "(none)" stops that part.
//
// PLUGGING DEVICES IN AND OUT
// The lists are polled every few seconds (see watchDevices), so devices that
// are plugged in or removed show up without pressing Refresh. A chosen device
// that disappears is replaced by the first entry of its list, and chosen again
// if it comes back.
//
// MENUS, HELP AND ABOUT
// The window has a native menu bar (see menu.go) with File, View and Help
// menus. Help opens the help text built into the program and About shows the
// version and details for bug reports. F1 also opens the help.
//
// FULL SCREEN
// Double click the picture, press F or F11, or use the button to enter full
// screen on the monitor the window is on. Escape, F, F11 or a double click
// leave it. Drag the window to another monitor before entering full screen
// to use that monitor.
//
// THREADING
// Fyne runs every callback on one main thread, and all state in App is only
// touched there. Slow work (asking ffmpeg for devices, opening audio devices)
// runs in goroutines that return their results with fyne.Do. A generation
// counter discards results that arrive after the user has already chosen
// something else.
//
// The settings that survive a restart (the chosen devices) are stored with
// Fyne's preferences, by device name.
package ui

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"my-capture/assets"
	"my-capture/internal/audio"
	"my-capture/internal/config"
	"my-capture/internal/video"
)

// -----------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------

const (
	// appName is the name shown in the menu bar, the About box and window
	// titles.
	appName = "My Capture"

	// appID identifies the application to the operating system and to
	// Fyne's preferences store.
	appID = "com.github.steveohara.my-capture"

	// noneLabel is the first entry of the video device and audio input
	// pickers. Choosing it stops that part.
	noneLabel = "(none)"

	// Preference keys under which the chosen devices are remembered.
	prefVideo  = "video.device"
	prefInput  = "audio.input"
	prefOutput = "audio.output"
	prefAspect = "video.aspect"

	// installHelp is shown when ffmpeg cannot be found.
	installHelp = "ffmpeg was not found, so video capture is unavailable. " +
		"Install it (macOS: brew install ffmpeg; Windows: winget install Gyan.FFmpeg) " +
		"or set " + config.EnvFFmpeg + " to its full path, then press Refresh."
)

// -----------------------------------------------------------------------
// App
// -----------------------------------------------------------------------

// App is the my-capture window and the state behind it.
//
// App must only be used from the Fyne main thread, except where a method
// says otherwise. Create it with Run, which does not return until the window
// is closed.
type App struct {
	// cfg is the configuration read from the environment.
	cfg config.Config

	// version is the program version shown in the About box.
	version string

	// fyneApp and win are the Fyne application and its single window.
	fyneApp fyne.App
	win     fyne.Window

	// ffmpeg is the path of the ffmpeg executable, valid when ffmpegErr is
	// nil.
	ffmpeg string

	// ffmpegErr is why ffmpeg could not be located, or nil.
	ffmpegErr error

	// engine plays the audio. It is nil when the audio back end could not
	// be started, in which case the picture still works.
	engine *audio.Engine

	// audioJobs carries audio device changes to one worker goroutine, so
	// they are applied in the order the user made them even though opening
	// a device can take a while.
	audioJobs chan func()

	// quitting is set when the window closes, and stops background
	// goroutines from touching Fyne after the main loop has ended.
	quitting atomic.Bool

	// The widgets making up the window.
	view        *VideoView
	videoSel    *widget.Select
	modeSel     *widget.Select
	inSel       *widget.Select
	outSel      *widget.Select
	aspectSel   *widget.Select
	delaySlider *widget.Slider
	delayLabel  *widget.Label
	status      *widget.Label
	controls    *fyne.Container
	content     *fyne.Container

	// videoDevs, modes, inputs and outputs are the lists behind the
	// pickers, in picker order. The "(none)" entry of the video and input
	// pickers is not in these slices, so picker index i is slice index i-1.
	videoDevs []video.Device
	modes     []video.Mode
	inputs    []audio.Device
	outputs   []audio.Device

	// settingModes is true while the mode picker is being filled in code, so
	// its change handler can tell that from a choice made by the user.
	settingModes bool

	// capture is the running video capture, or nil.
	capture *video.Capture

	// gen is the generation counter described in the package comment. It is
	// incremented whenever the video selection changes.
	gen int

	// restored is true once the saved device choices have been applied.
	restored bool

	// wanted maps a picker to the name of a device that was chosen and then
	// unplugged. If it reappears it is chosen again. See updateSelect.
	wanted map[*widget.Select]string

	// fallingBack is true while updateSelect switches a picker to its first
	// entry because the chosen device vanished.
	fallingBack bool

	// audioPolling and videoPolling are true while a background device check
	// is running, so checks never overlap.
	audioPolling bool
	videoPolling bool

	// mainMenu is the menu bar, nil until buildMenus has installed it, and
	// aspectItems are the entries of its Aspect submenu, in AspectModes order.
	mainMenu    *fyne.MainMenu
	aspectItems []*fyne.MenuItem

	// fullscreen is true while the window is in full screen. controlsHidden
	// is true when the user has hidden the controls in windowed mode. The
	// controls are shown only when neither is true. Main thread only.
	fullscreen     bool
	controlsHidden bool

	// controlsItem is the View menu entry that hides and shows the controls.
	// Its label changes with the state.
	controlsItem *fyne.MenuItem

	// helpWin is the Help window while it is open, or nil.
	helpWin fyne.Window

	// done is closed at shutdown to stop watchDevices.
	done chan struct{}
}

// Run opens the window and blocks until it is closed. version is the program
// version shown in the About box.
func Run(cfg config.Config, version string) {
	a := &App{
		cfg:       cfg,
		version:   version,
		audioJobs: make(chan func(), 64),
		wanted:    map[*widget.Select]string{},
		done:      make(chan struct{}),
	}
	platformInit(appName, version, assets.IconMacPNG, cfg.NoRelaunch)
	a.fyneApp = app.NewWithID(appID)
	a.fyneApp.SetIcon(fyne.NewStaticResource("icon.png", assets.IconPNG))
	a.win = a.fyneApp.NewWindow(appName)

	a.ffmpeg, a.ffmpegErr = video.FindFFmpeg(cfg.FFmpegPath)

	var engineErr error
	a.engine, engineErr = audio.NewEngine(cfg.AudioDelayMs, a.onAudioStopped)

	a.build()
	if a.ffmpegErr != nil {
		a.setStatus(installHelp)
	} else if engineErr != nil {
		a.setStatus("Audio is unavailable: " + engineErr.Error())
	}

	// Lifetime: runs until audioJobs is closed in shutdown.
	go func() {
		for job := range a.audioJobs {
			job()
		}
	}()

	go a.watchDevices(a.done)
	a.refreshDevices()
	a.win.Resize(fyne.NewSize(1100, 760))
	a.win.ShowAndRun()
	a.shutdown()
}

// shutdown stops everything after the window has closed.
func (a *App) shutdown() {
	a.quitting.Store(true)
	close(a.done)
	close(a.audioJobs)
	a.stopVideo()
	if a.engine != nil {
		a.engine.Close()
	}
}

// ui runs fn on the Fyne main thread, unless the program is exiting. It is
// safe to call from any goroutine.
func (a *App) ui(fn func()) {
	if a.quitting.Load() {
		return
	}
	fyne.Do(fn)
}

// -----------------------------------------------------------------------
// Building the window
// -----------------------------------------------------------------------

// build creates the widgets and lays them out.
func (a *App) build() {
	a.view = NewVideoView(a.toggleFullscreen)

	a.videoSel = widget.NewSelect([]string{noneLabel}, func(string) { a.onVideoChanged() })
	a.modeSel = widget.NewSelect(nil, func(string) { a.onModeChanged() })
	a.modeSel.Disable()
	a.inSel = widget.NewSelect([]string{noneLabel}, func(string) { a.onInputChanged() })
	a.outSel = widget.NewSelect(nil, func(string) { a.onOutputChanged() })
	a.videoSel.PlaceHolder = "Select a video device"
	a.inSel.PlaceHolder = "Select an audio input"
	a.outSel.PlaceHolder = "Select an audio output"

	a.delayLabel = widget.NewLabel("")
	a.delaySlider = widget.NewSlider(config.MinAudioDelayMs, config.MaxAudioDelayMs)
	a.delaySlider.Step = 10
	a.delaySlider.SetValue(float64(a.cfg.AudioDelayMs))
	a.updateDelayLabel()
	a.delaySlider.OnChanged = func(float64) { a.updateDelayLabel() }
	a.delaySlider.OnChangeEnded = a.onDelayChanged

	refresh := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), a.refreshDevices)
	fullscreen := widget.NewButtonWithIcon("Full screen", theme.ViewFullScreenIcon(), a.toggleFullscreen)
	hide := widget.NewButtonWithIcon("Hide controls", theme.VisibilityOffIcon(), a.toggleControls)

	a.status = widget.NewLabel("")
	a.status.Wrapping = fyne.TextWrapWord

	labelled := func(text string, obj fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(text), nil, obj)
	}
	aspectNames := make([]string, len(AspectModes))
	for i, m := range AspectModes {
		aspectNames[i] = m.Name
	}
	a.aspectSel = widget.NewSelect(aspectNames, func(string) { a.onAspectChanged() })
	a.aspectSel.SetSelected(a.savedAspect(aspectNames))

	delay := container.NewBorder(nil, nil, a.delayLabel, nil, a.delaySlider)
	pickers := container.NewGridWithColumns(2,
		labelled("Video device", a.videoSel), labelled("Mode", a.modeSel),
		labelled("Audio input", a.inSel), labelled("Audio output", a.outSel),
		labelled("Aspect", a.aspectSel), delay)
	statusRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(refresh, hide, fullscreen), a.status)

	// The window itself is unpadded (see below), so the controls carry their
	// own padding and the picture runs right up to the window edges.
	a.controls = container.NewPadded(container.NewVBox(pickers, statusRow))
	a.content = container.NewBorder(a.controls, nil, nil, nil, a.view)

	// Fyne's window padding can not be switched at run time: SetPadded moves
	// the content but does not resize it, which left a white strip, the width
	// of two paddings, down the right and bottom of a full screen picture.
	// So the window is never padded.
	a.win.SetPadded(false)
	a.win.SetContent(a.content)
	a.buildMenus()

	a.win.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		switch e.Name {
		case fyne.KeyF, fyne.KeyF11:
			a.toggleFullscreen()
		case fyne.KeyA:
			a.cycleAspect()
		case fyne.KeyC:
			a.toggleControls()
		case fyne.KeyF1:
			a.showHelp()
		case fyne.KeyEscape:
			if a.fullscreen {
				a.setFullscreen(false)
			}
		}
	})
}

// savedAspect returns the aspect mode remembered from the last run, or the
// first mode when there is none or it no longer exists.
func (a *App) savedAspect(names []string) string {
	saved := a.fyneApp.Preferences().String(prefAspect)
	if slices.Contains(names, saved) {
		return saved
	}
	return names[0]
}

// onAspectChanged applies the aspect mode chosen in the picker.
func (a *App) onAspectChanged() {
	a.unfocus()
	if i := a.aspectSel.SelectedIndex(); i >= 0 {
		a.view.SetAspect(AspectModes[i])
	}
	a.remember(prefAspect, a.aspectSel)
	a.syncAspectMenu()
}

// cycleAspect moves to the next aspect mode. It is bound to the A key, so
// the shape can be changed in full screen, when the picker is hidden.
func (a *App) cycleAspect() {
	next := (a.aspectSel.SelectedIndex() + 1) % len(AspectModes)
	a.aspectSel.SetSelectedIndex(next)
}

// setStatus shows a message below the controls.
func (a *App) setStatus(msg string) {
	a.status.SetText(msg)
}

// updateDelayLabel shows the slider's current value next to it.
func (a *App) updateDelayLabel() {
	a.delayLabel.SetText(fmt.Sprintf("Audio delay: %4d ms", int(a.delaySlider.Value)))
}

// toggleFullscreen switches between windowed and full screen.
func (a *App) toggleFullscreen() {
	a.setFullscreen(!a.fullscreen)
}

// setFullscreen enters or leaves full screen. The controls are hidden in full
// screen so the picture gets the whole monitor, and come back afterwards
// unless the user had hidden them.
func (a *App) setFullscreen(on bool) {
	a.win.SetFullScreen(on)
	a.fullscreen = on
	a.applyControls()
}

// toggleControls hides the controls strip, or shows it again, in windowed
// mode. In full screen the controls are always hidden, so this does nothing
// there rather than quietly changing what happens on leaving full screen.
func (a *App) toggleControls() {
	if a.fullscreen {
		return
	}
	a.controlsHidden = !a.controlsHidden
	a.applyControls()
}

// applyControls shows or hides the controls strip to match the full screen
// and hidden flags, and brings the View menu entry into line.
func (a *App) applyControls() {
	if a.fullscreen || a.controlsHidden {
		a.controls.Hide()
		a.unfocus() // a hidden control that keeps the focus would swallow the keys
	} else {
		a.controls.Show()
	}
	a.content.Refresh()
	a.syncControlsMenu()
}

// unfocus takes the keyboard focus off whichever control has it. A focused
// control receives the key presses itself, which would stop the window's
// shortcut keys (F, A, C, Escape and F1) working after the user has used a
// dropdown or a button.
func (a *App) unfocus() {
	if a.content != nil {
		a.win.Canvas().Unfocus()
	}
}

// -----------------------------------------------------------------------
// Device lists
// -----------------------------------------------------------------------

// refreshDevices asks the video and audio back ends for their devices and
// updates the pickers. It keeps the current choices where the devices still
// exist. It runs at start up and when the Refresh button is pressed; the
// periodic checks in watchDevices cover plugging and unplugging in between.
func (a *App) refreshDevices() {
	a.unfocus()
	a.findFFmpegAgain()
	if a.capture == nil {
		a.setStatus("Looking for devices...")
	}

	// Read the fields on the main thread; the goroutine gets copies.
	ffmpeg, haveFFmpeg, engine := a.ffmpeg, a.ffmpegErr == nil, a.engine
	go func() {
		var vdevs []video.Device
		var verr, ierr, oerr error
		if haveFFmpeg {
			vdevs, verr = video.ListDevices(context.Background(), ffmpeg)
		}
		var ins, outs []audio.Device
		if engine != nil {
			ins, ierr = engine.Inputs()
			outs, oerr = engine.Outputs()
		}
		a.ui(func() {
			// A list that could not be read is left as it was, rather than
			// being emptied by a temporary failure.
			if haveFFmpeg && verr == nil {
				a.setVideoDevices(vdevs)
			}
			if engine != nil && ierr == nil && oerr == nil {
				a.setAudioDevices(ins, outs)
			}
			a.afterRefresh(firstError(verr, ierr, oerr))
		})
	}()
}

// afterRefresh restores the previous run's choices the first time the lists
// are filled, and then shows a hint matching the state of the window.
// problem, if not nil, is the first error met while discovering devices.
func (a *App) afterRefresh(problem error) {
	if !a.restored {
		a.restored = true
		a.restoreSelections()
	}

	switch {
	case problem != nil:
		a.setStatus("Could not list every device: " + problem.Error())
	case a.ffmpegErr != nil:
		a.setStatus(installHelp)
	case len(a.videoDevs) == 0:
		a.setStatus("No video capture device found. Connect one and it will appear here.")
	case a.videoSel.SelectedIndex() <= 0:
		a.setStatus("Choose a video device.")
	}
}

// findFFmpegAgain looks for ffmpeg again if it was missing, so installing it
// while the program runs works without a restart.
func (a *App) findFFmpegAgain() {
	if a.ffmpegErr == nil {
		return
	}
	if path, err := video.FindFFmpeg(a.cfg.FFmpegPath); err == nil {
		a.ffmpeg, a.ffmpegErr = path, nil
		a.setStatus("ffmpeg found.")
	}
}

// setVideoDevices installs a video device list into the picker.
func (a *App) setVideoDevices(devs []video.Device) {
	// Always keep the newest list, even when the names are unchanged,
	// because on macOS a device's index can change while its name does not.
	a.videoDevs = devs
	a.updateSelect(a.videoSel, withNone(uniqueNames(videoNames(devs))))
}

// setAudioDevices installs audio input and output lists into the pickers.
func (a *App) setAudioDevices(ins, outs []audio.Device) {
	a.inputs, a.outputs = ins, outs
	a.updateSelect(a.inSel, withNone(uniqueNames(audioNames(ins))))
	a.updateSelect(a.outSel, uniqueNames(audioNames(outs)))
}

// -----------------------------------------------------------------------
// Watching for plugged and unplugged devices
// -----------------------------------------------------------------------

const (
	// audioPollInterval is how often the audio device lists are checked.
	// Listing audio devices is cheap, so this can be frequent.
	audioPollInterval = 2 * time.Second

	// videoPollInterval is how often the video device list is checked. Each
	// check runs ffmpeg, so this is slower.
	videoPollInterval = 4 * time.Second
)

// watchDevices triggers the periodic device checks until done is closed. It
// only keeps time; the checks themselves start on the main thread, which is
// where the state they compare against lives.
//
// Lifetime: started once by Run, ends when shutdown closes done.
func (a *App) watchDevices(done <-chan struct{}) {
	audioTick := time.NewTicker(audioPollInterval)
	videoTick := time.NewTicker(videoPollInterval)
	defer audioTick.Stop()
	defer videoTick.Stop()
	for {
		select {
		case <-done:
			return
		case <-audioTick.C:
			a.ui(a.pollAudio)
		case <-videoTick.C:
			a.ui(a.pollVideo)
		}
	}
}

// pollAudio checks the audio devices in the background and updates the
// pickers if they changed. A check that fails is ignored, and the lists keep
// their last known contents.
func (a *App) pollAudio() {
	engine := a.engine
	if engine == nil || a.audioPolling {
		return
	}
	a.audioPolling = true
	go func() {
		ins, ierr := engine.Inputs()
		outs, oerr := engine.Outputs()
		a.ui(func() {
			a.audioPolling = false
			if ierr == nil && oerr == nil {
				a.setAudioDevices(ins, outs)
			}
		})
	}()
}

// pollVideo checks the video devices in the background and updates the
// picker if they changed. At most one check runs at a time, because a slow
// ffmpeg must not pile up processes.
func (a *App) pollVideo() {
	a.findFFmpegAgain()
	if a.ffmpegErr != nil || a.videoPolling {
		return
	}
	a.videoPolling = true
	ffmpeg := a.ffmpeg
	go func() {
		devs, err := video.ListDevices(context.Background(), ffmpeg)
		a.ui(func() {
			a.videoPolling = false
			if err == nil {
				a.setVideoDevices(devs)
			}
		})
	}()
}

// updateSelect replaces the options of sel, if they have changed.
//
// If the current choice has vanished (the device was unplugged), the first
// option is chosen, which runs the change handler and so stops that part. The
// vanished name is remembered in a.wanted, and if that device comes back the
// choice is restored, so a Bluetooth headset that drops out and reconnects
// picks up where it left off. A choice that still exists is left alone.
func (a *App) updateSelect(sel *widget.Select, options []string) {
	if slices.Equal(sel.Options, options) {
		return // unchanged: avoid redrawing, which would close an open list
	}
	current := sel.Selected
	sel.SetOptions(options)
	if len(options) == 0 {
		return
	}

	if want := a.wanted[sel]; want != "" && want != current && slices.Contains(options, want) {
		sel.SetSelected(want) // the handler clears the wish
		return
	}
	if current == "" || slices.Contains(options, current) {
		return
	}

	a.wanted[sel] = current
	a.fallingBack = true
	sel.SetSelected(options[0])
	a.fallingBack = false
	a.setStatus(fmt.Sprintf("%q was disconnected.", current))
}

// remember stores the user's choice in sel under key, and forgets any
// device they were waiting for. It does nothing while updateSelect is
// falling back after an unplug, because that is not a choice the user made
// and must not overwrite what is remembered.
func (a *App) remember(key string, sel *widget.Select) {
	if a.fallingBack {
		return
	}
	delete(a.wanted, sel)
	a.fyneApp.Preferences().SetString(key, selectedOrEmpty(sel))
}

// restoreSelections applies the devices chosen in the previous run, where
// they are still present. The output always gets a choice, falling back to
// the system default, so sound plays as soon as an input is chosen.
func (a *App) restoreSelections() {
	prefs := a.fyneApp.Preferences()

	if len(a.outSel.Options) > 0 {
		a.selectIfPresent(a.outSel, prefs.String(prefOutput), a.outSel.Options[0])
	}
	a.selectIfPresent(a.inSel, prefs.String(prefInput), noneLabel)
	a.selectIfPresent(a.videoSel, prefs.String(prefVideo), noneLabel)
}

// selectIfPresent chooses name in sel if it is one of the options. If it is
// not (the device is not plugged in yet), fallback is chosen instead, and the
// name is kept in a.wanted and in the saved preferences, so the device is
// chosen as soon as it appears.
func (a *App) selectIfPresent(sel *widget.Select, name, fallback string) {
	if name != "" && slices.Contains(sel.Options, name) {
		sel.SetSelected(name)
		return
	}
	if name != "" {
		a.wanted[sel] = name
	}
	if fallback != "" {
		a.fallingBack = true // keep the remembered name in the preferences
		sel.SetSelected(fallback)
		a.fallingBack = false
	}
}

// -----------------------------------------------------------------------
// Video
// -----------------------------------------------------------------------

// onVideoChanged runs when the user picks a video device. It stops the
// current capture, reads the new device's modes in the background, and then
// starts capturing.
func (a *App) onVideoChanged() {
	a.unfocus()
	a.gen++
	gen := a.gen
	a.stopVideo()
	a.view.Clear()
	a.clearModes()

	i := a.videoSel.SelectedIndex() - 1
	a.remember(prefVideo, a.videoSel)
	if i < 0 || i >= len(a.videoDevs) {
		a.setStatus("No video device selected.")
		return
	}
	dev := a.videoDevs[i]
	a.setStatus("Reading the modes of " + dev.Name + "...")

	go func() {
		modes, err := video.ListModes(context.Background(), a.ffmpeg, dev)
		a.ui(func() {
			if gen != a.gen {
				return // the user has chosen something else meanwhile
			}
			if err != nil {
				// Not fatal: the device default is still tried below.
				fmt.Fprintf(os.Stderr, "warning: read modes of %q: %v\n", dev.Name, err)
			}
			a.applyModes(dev, modes)
		})
	}()
}

// clearModes empties the mode picker without running its change handler.
func (a *App) clearModes() {
	a.settingModes = true
	a.modes = nil
	a.modeSel.Options = nil
	a.modeSel.Selected = ""
	a.modeSel.Refresh()
	a.modeSel.Disable()
	a.settingModes = false
}

// applyModes fills the mode picker, with the best mode first and the device
// default last, picks the first entry and starts capturing.
func (a *App) applyModes(dev video.Device, modes []video.Mode) {
	options := make([]string, 0, len(modes)+1)
	for _, m := range modes {
		options = append(options, m.String())
	}
	options = append(options, video.Mode{}.String())

	a.settingModes = true
	a.modes = modes
	a.modeSel.SetOptions(options)
	a.modeSel.SetSelectedIndex(0)
	a.modeSel.Enable()
	a.settingModes = false

	a.pairAudioInput(dev)
	a.startVideo()
}

// onModeChanged runs when the user picks a different mode.
func (a *App) onModeChanged() {
	if a.settingModes {
		return
	}
	a.unfocus()
	a.startVideo()
}

// startVideo (re)starts the capture with the selected device and mode.
func (a *App) startVideo() {
	a.gen++
	gen := a.gen
	a.stopVideo()

	i := a.videoSel.SelectedIndex() - 1
	if i < 0 || i >= len(a.videoDevs) || a.ffmpegErr != nil {
		return
	}
	dev := a.videoDevs[i]

	mode := video.Mode{}
	if j := a.modeSel.SelectedIndex(); j >= 0 && j < len(a.modes) {
		mode = a.modes[j]
	}

	c, err := video.Start(context.Background(), video.Options{
		FFmpegPath: a.ffmpeg,
		Device:     dev,
		Mode:       mode,
		Debug:      a.cfg.Debug,
	}, a.view.SetFrame, func(err error) {
		a.ui(func() {
			if gen != a.gen {
				return
			}
			a.capture = nil
			a.view.Clear()
			a.setStatus("Video stopped: " + err.Error())
		})
	})
	if err != nil {
		a.setStatus("Could not start video: " + err.Error())
		return
	}
	a.capture = c
	a.setStatus(fmt.Sprintf("Showing %s, %s.", dev.Name, mode))
}

// stopVideo ends the running capture, if any.
func (a *App) stopVideo() {
	if a.capture != nil {
		a.capture.Stop()
		a.capture = nil
	}
}

// pairAudioInput chooses the audio input whose name matches the video
// device, if no audio input is chosen yet. Capture devices usually present
// their sound as a separate input with a similar name.
func (a *App) pairAudioInput(dev video.Device) {
	if a.inSel.SelectedIndex() > 0 {
		return
	}
	if i := matchName(dev.Name, audioNames(a.inputs)); i >= 0 {
		a.inSel.SetSelectedIndex(i + 1)
	}
}

// -----------------------------------------------------------------------
// Audio
// -----------------------------------------------------------------------

// onInputChanged runs when the user picks an audio input.
func (a *App) onInputChanged() {
	a.unfocus()
	a.remember(prefInput, a.inSel)
	if a.engine == nil {
		return
	}
	var dev *audio.Device
	if i := a.inSel.SelectedIndex() - 1; i >= 0 && i < len(a.inputs) {
		d := a.inputs[i]
		dev = &d
	}
	a.audioJob(func() error { return a.engine.SetInput(dev) })
}

// onOutputChanged runs when the user picks an audio output.
func (a *App) onOutputChanged() {
	a.unfocus()
	a.remember(prefOutput, a.outSel)
	if a.engine == nil {
		return
	}
	var dev *audio.Device
	if i := a.outSel.SelectedIndex(); i >= 0 && i < len(a.outputs) {
		d := a.outputs[i]
		dev = &d
	}
	a.audioJob(func() error { return a.engine.SetOutput(dev) })
}

// onDelayChanged runs when the user lets go of the audio delay slider.
func (a *App) onDelayChanged(v float64) {
	a.unfocus()
	if a.engine == nil {
		return
	}
	ms := int(v)
	a.audioJob(func() error {
		a.engine.SetDelay(ms)
		return nil
	})
}

// audioJob queues an audio change for the worker goroutine and reports its
// failure in the status line.
func (a *App) audioJob(job func() error) {
	a.audioJobs <- func() {
		if err := job(); err != nil {
			a.ui(func() { a.setStatus("Audio problem: " + err.Error()) })
		}
	}
}

// onAudioStopped is called by the engine, on an audio thread, when a device
// stops by itself, as a Bluetooth headset does when it switches off.
func (a *App) onAudioStopped(role string) {
	a.ui(func() {
		a.setStatus("The audio " + role + " device stopped. Choose a device again, or press Refresh.")
	})
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// selectedOrEmpty returns the choice of sel, or "" when it is the "(none)"
// entry, so that "nothing" is stored as an empty preference.
func selectedOrEmpty(sel *widget.Select) string {
	if sel.Selected == noneLabel {
		return ""
	}
	return sel.Selected
}

// withNone puts the "(none)" entry in front of names.
func withNone(names []string) []string {
	return append([]string{noneLabel}, names...)
}

// videoNames returns the display names of devs.
func videoNames(devs []video.Device) []string {
	names := make([]string, len(devs))
	for i, d := range devs {
		names[i] = d.Name
	}
	return names
}

// audioNames returns the display names of devs.
func audioNames(devs []audio.Device) []string {
	names := make([]string, len(devs))
	for i, d := range devs {
		names[i] = d.Name
	}
	return names
}

// uniqueNames makes every name distinct by adding " (2)", " (3)" and so on
// to repeats, so two identical capture cards can be told apart in a picker
// and so the picker index always identifies one device.
func uniqueNames(names []string) []string {
	seen := map[string]int{}
	out := make([]string, len(names))
	for i, n := range names {
		seen[n]++
		if seen[n] > 1 {
			n = fmt.Sprintf("%s (%d)", n, seen[n])
		}
		out[i] = n
	}
	return out
}

// matchName returns the index in candidates of the first name that equals
// target or contains it or is contained by it, ignoring case, or -1.
func matchName(target string, candidates []string) int {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "" {
		return -1
	}
	for i, c := range candidates {
		c = strings.ToLower(strings.TrimSpace(c))
		if c != "" && (strings.Contains(c, t) || strings.Contains(t, c)) {
			return i
		}
	}
	return -1
}

// firstError returns the first non-nil error, or nil.
func firstError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
