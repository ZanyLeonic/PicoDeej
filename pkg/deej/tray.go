package deej

import (
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/ncruces/zenity"
	"github.com/zanyleonic/picodeej/pkg/deej/icon"
	"github.com/zanyleonic/picodeej/pkg/deej/util"
)

func (d *Deej) initializeTray(onDone func()) {
	logger := d.logger.Named("tray")

	onReady := func() {
		logger.Debug("Tray instance ready")

		systray.SetTemplateIcon(icon.DeejLogo, icon.DeejLogo)
		systray.SetTitle("deej")
		systray.SetTooltip("deej")

		editConfig := systray.AddMenuItem("Edit configuration", "Open config file with notepad")
		editConfig.SetIcon(icon.EditConfig)

		refreshSessions := systray.AddMenuItem("Re-scan audio sessions", "Manually refresh audio sessions if something's stuck")
		refreshSessions.SetIcon(icon.RefreshSessions)

		uploadImage := systray.AddMenuItem("Upload image", "Upload an image to the Deej's display")
		uploadImage.SetIcon(icon.UploadImage)

		if d.version != "" {
			systray.AddSeparator()
			versionInfo := systray.AddMenuItem(d.version, "")
			versionInfo.Disable()
		}

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop deej and quit")

		// wait on things to happen
		go func() {
			for {
				select {

				// quit
				case <-quit.ClickedCh:
					logger.Info("Quit menu item clicked, stopping")

					d.signalStop()

				// edit config
				case <-editConfig.ClickedCh:
					logger.Info("Edit config menu item clicked, opening config for editing")

					editor := "notepad.exe"
					if util.Linux() {
						editor = "gedit"
					}

					if err := util.OpenExternal(logger, editor, userConfigFilepath); err != nil {
						logger.Warnw("Failed to open config file for editing", "error", err)
					}

				// refresh sessions
				case <-refreshSessions.ClickedCh:
					logger.Info("Refresh sessions menu item clicked, triggering session map refresh")

					// performance: the reason that forcing a refresh here is okay is that users can't spam the
					// right-click -> select-this-option sequence at a rate that's meaningful to performance
					d.sessions.refreshSessions(true)

				case <-uploadImage.ClickedCh:
					logger.Info("Upload image menu item, clicked, opening dialog")
					if d.serial.transferInProgress {
						d.notifier.Notify("Image Upload in progress", "Only one transfer can happen at once.")
						continue
					}

					d.serial.transferInProgress = true

					file, err := zenity.SelectFile(
						zenity.Filename(``),
						zenity.FileFilters{
							{Name: "Deej Compatible Formats", Patterns: []string{"*.png", "*.zip"}, CaseFold: true},
							{Name: "Portable Network Graphic", Patterns: []string{"*.png"}, CaseFold: true},
							{Name: "Animated Image Set", Patterns: []string{"*.zip"}, CaseFold: true},
						})

					if err != nil {
						logger.Errorw("Failed to create zenity file picker!", "error", err)
						continue
					}

					ext := filepath.Ext(file)
					logger.Debugw("Selected a file using the file picker", "path", file)

					if ext == ".png" {
						err = d.serial.StartImageUpload(logger, file)
					} else if ext == ".zip" {
						err = d.serial.StartAnimatedUpload(logger, file)
					} else {
						logger.Errorw("User did not select a correct type of file.", "ext", ext)
						d.notifier.Notify("Invalid file selected", "Image upload only supported static PNGs or Animated Image Sets in ZIP files.")
						continue
					}

					if err != nil {
						logger.Errorw("Cannot upload selected image", "error", err)
						d.notifier.Notify("Error Starting Image Upload", "Please ensure all previous transfers have completed and dialogs closed, before starting another.")
						d.serial.transferInProgress = false
						continue
					}
				}
			}
		}()

		// actually start the main runtime
		onDone()
	}

	onExit := func() {
		logger.Debug("Tray exited")
	}

	// start the tray icon
	logger.Debug("Running in tray")
	systray.Run(onReady, onExit)
}

func (d *Deej) stopTray() {
	d.logger.Debug("Quitting tray")
	systray.Quit()
}
