// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file_browser

import (
	"fmt"
	"strings"
	"time"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// speedSample captures a data point for sliding-window speed calculation.
type speedSample struct {
	time  time.Time
	bytes int64
}

const (
	// maxSpeedSamples is the number of samples retained for speed averaging.
	maxSpeedSamples = 5

	// completedColor is the color used when a transfer completes successfully.
	completedColor = tcell.Color33

	// progressPausedColor is the gray color for paused progress state.
	progressPausedColor = tcell.Color245
)

// cancelWarningColor is the gold color for "Cancel transfer?" prompt.
var cancelWarningColor = tcell.NewRGBColor(255, 215, 0) // #FFD700 gold

// cancelConfirmedColor is the red color for "Transfer canceled" text.
var cancelConfirmedColor = tcell.NewRGBColor(255, 107, 107) // #FF6B6B red

// conflictWarningColor is the orange color for "File already exists:" prompt.
var conflictWarningColor = tcell.NewRGBColor(255, 165, 0) // #FFA500 orange

// modalMode enumerates the display modes of the TransferModal.
type modalMode int

const (
	modeProgress       modalMode = iota // Normal progress display (existing)
	modeCancelConfirm                   // Cancel confirmation dialog (new)
	modeConflictDialog                  // Conflict resolution dialog (Plan 02)
	modeSummary                         // Transfer complete/canceled summary (existing)
)

// TransferModal is a full-screen overlay component that displays file transfer progress.
// It renders a progress bar, file name, transfer speed, ETA, and supports
// directory transfer summaries. It embeds *tview.Box for background rendering
// and implements manual Draw for precise layout control.
//
// Multi-mode system: the modal switches between progress/cancelConfirm/conflictDialog/summary
// modes via the mode field. Each mode renders different content in the Draw() method.
//
// All UI updates must go through app.QueueUpdateDraw() for thread safety.
type TransferModal struct {
	*tview.Box
	app *tview.Application
	bar *ProgressBar

	// display state
	fileLabel    string      // "Uploading: filename.txt"
	infoLine     string      // "67%  2.3 MB/s"
	etaLine      string      // "ETA: 0m 12s"
	summaryLine  string      // "Transferred 8/10 files, 2 failed"
	summaryColor tcell.Color // color for summary text (white for success, red for canceled)

	// mode system
	mode            modalMode
	cancelConfirmed bool

	// speed tracking
	speedSamples []speedSample

	// modal state
	visible   bool
	onDismiss func()

	// conflict dialog state
	conflictFileInfo string // "filename.txt (1.2M, 2024-03-15)"
	conflictActionCh chan domain.ConflictAction
}

// NewTransferModal creates a new TransferModal overlay component.
func NewTransferModal(app *tview.Application) *TransferModal {
	tm := &TransferModal{
		Box:          tview.NewBox().SetBorderPadding(2, 2, 5, 5),
		app:          app,
		bar:          NewProgressBar(),
		visible:      false,
		speedSamples: make([]speedSample, 0, maxSpeedSamples),
		summaryColor: tcell.Color248,
	}
	tm.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return tm
}

// Draw renders the transfer modal to the screen.
// Layout and content depend on the current modalMode:
//
//	modeProgress: progress bar, speed, ETA
//	modeCancelConfirm: centered "Cancel transfer?" prompt
//	modeSummary: transfer summary with "Press any key to close"
func (tm *TransferModal) Draw(screen tcell.Screen) {
	if !tm.visible {
		return
	}
	tm.Box.DrawForSubclass(screen, tm)

	x, y, width, height := tm.GetInnerRect()

	switch tm.mode {
	case modeCancelConfirm:
		tm.drawCancelConfirm(screen, x, y, width, height)
	case modeConflictDialog:
		tm.drawConflictDialog(screen, x, y, width, height)
	case modeSummary:
		tm.drawSummary(screen, x, y, width, height)
	default:
		tm.drawProgress(screen, x, y, width, height)
	}
}

// drawProgress renders the normal progress display.
func (tm *TransferModal) drawProgress(screen tcell.Screen, x, y, width, height int) {
	// Row 1: file name label
	row1 := y + 1
	tview.Print(screen, tm.fileLabel, x, row1, width, tview.AlignCenter, tcell.Color255)

	// Row 3: progress bar + percentage
	row3 := y + 3
	barStr := tm.bar.String()
	pct := ""
	if tm.bar.total > 0 {
		p := float64(tm.bar.current) / float64(tm.bar.total) * 100
		if p > 100 {
			p = 100
		}
		pct = fmt.Sprintf(" %.0f%%", p)
	}
	barWithPct := barStr + pct
	tview.Print(screen, barWithPct, x, row3, width, tview.AlignCenter, tcell.Color248)

	// Row 4: speed
	row4 := y + 4
	tview.Print(screen, tm.infoLine, x, row4, width, tview.AlignCenter, tcell.Color245)

	// Row 5: ETA
	row5 := y + 5
	tview.Print(screen, tm.etaLine, x, row5, width, tview.AlignCenter, tcell.Color245)
}

// drawCancelConfirm renders the cancel confirmation dialog.
// Layout per 03-UI-SPEC Mode 2: centered prompt with options.
func (tm *TransferModal) drawCancelConfirm(screen tcell.Screen, x, y, width, height int) {
	// Center vertically within the modal inner area
	row := y + height/2 - 1
	tview.Print(screen, "Cancel transfer?", x, row, width, tview.AlignCenter, cancelWarningColor)
	tview.Print(screen, "[y] Yes  [n] No", x, row+1, width, tview.AlignCenter, tcell.Color255)

	// Footer hint
	tview.Print(screen, "Press Esc to continue transfer", x, y+height-2, width, tview.AlignCenter, tcell.Color245)
}

// drawConflictDialog renders the conflict resolution dialog.
// Layout per 03-UI-SPEC Mode 3: file info + three options.
func (tm *TransferModal) drawConflictDialog(screen tcell.Screen, x, y, width, height int) {
	row := y + 1
	tview.Print(screen, "File already exists:", x, row, width, tview.AlignCenter, conflictWarningColor)
	row++
	tview.Print(screen, tm.conflictFileInfo, x, row, width, tview.AlignCenter, tcell.Color255)
	row += 2
	tview.Print(screen, "[o] Overwrite  [s] Skip  [r] Rename", x, row, width, tview.AlignCenter, tcell.Color255)
}

// drawSummary renders the transfer summary (completion or canceled).
func (tm *TransferModal) drawSummary(screen tcell.Screen, x, y, width, height int) {
	// Row 1: summary line (colored appropriately)
	row1 := y + 1
	tview.Print(screen, tm.summaryLine, x, row1, width, tview.AlignCenter, tm.summaryColor)

	// Footer
	tm.drawSummaryFooter(screen, x, y, width, height)
}

// drawSummaryFooter renders the bottom portion of the summary view
// with a "Press any key to close" hint.
func (tm *TransferModal) drawSummaryFooter(screen tcell.Screen, x, y, width, height int) {
	hint := "Press any key to close"
	footerRow := y + height - 2
	tview.Print(screen, hint, x, footerRow, width, tview.AlignCenter, tcell.Color245)
}

// Show displays the modal as a full-screen overlay in progress mode.
// direction should be "Uploading" or "Downloading".
func (tm *TransferModal) Show(direction, filename string) {
	tm.visible = true
	tm.mode = modeProgress
	tm.cancelConfirmed = false

	title := fmt.Sprintf(" %s ", direction)
	if filename != "" {
		title = fmt.Sprintf(" %s %s ", direction, filename)
	}
	tm.SetTitle(title)

	// Reset state
	tm.bar = NewProgressBar()
	tm.speedSamples = tm.speedSamples[:0]
	tm.fileLabel = fmt.Sprintf("%s: %s", direction, filename)
	tm.infoLine = ""
	tm.etaLine = ""
	tm.summaryLine = ""
}

// ShowCancelConfirm switches the modal to cancel confirmation mode.
func (tm *TransferModal) ShowCancelConfirm() {
	tm.mode = modeCancelConfirm
	tm.cancelConfirmed = false
}

// ResumeProgress returns the modal from cancel confirmation back to progress mode.
func (tm *TransferModal) ResumeProgress() {
	tm.mode = modeProgress
	tm.cancelConfirmed = false
}

// InCancelConfirm returns whether the modal is currently showing the cancel confirmation.
func (tm *TransferModal) InCancelConfirm() bool {
	return tm.mode == modeCancelConfirm
}

// SetCancelConfirmed sets the cancel confirmed flag.
func (tm *TransferModal) SetCancelConfirmed(v bool) {
	tm.cancelConfirmed = v
}

// IsCanceled returns whether the user has confirmed transfer cancellation.
func (tm *TransferModal) IsCanceled() bool {
	return tm.cancelConfirmed
}

// ShowCanceledSummary switches the modal to summary mode with "Transfer canceled" text.
func (tm *TransferModal) ShowCanceledSummary() {
	tm.mode = modeSummary

	// Clear per-file state
	tm.bar = NewProgressBar()
	tm.infoLine = ""
	tm.etaLine = ""
	tm.fileLabel = ""
	tm.conflictFileInfo = ""
	tm.conflictActionCh = nil

	tm.summaryLine = "Transfer canceled"
	tm.summaryColor = cancelConfirmedColor
	tm.SetTitle(" Transfer Complete ")
}

// ShowConflict switches the modal to conflict dialog mode.
// actionCh is a buffered channel (capacity 1) used to send the user's choice
// back to the transfer goroutine. The goroutine blocks on <-actionCh.
func (tm *TransferModal) ShowConflict(fileName, fileInfo string, actionCh chan domain.ConflictAction) {
	tm.mode = modeConflictDialog
	tm.conflictFileInfo = fmt.Sprintf("%s (%s)", fileName, fileInfo)
	tm.conflictActionCh = actionCh
}

// InConflictDialog returns whether the modal is currently showing the conflict dialog.
func (tm *TransferModal) InConflictDialog() bool {
	return tm.mode == modeConflictDialog
}

// ShowSummary displays a directory transfer summary instead of per-file progress.
func (tm *TransferModal) ShowSummary(transferred, failed int, failedFiles []string) {
	tm.mode = modeSummary
	tm.summaryColor = tcell.Color248

	// Clear per-file state
	tm.bar = NewProgressBar()
	tm.infoLine = ""
	tm.etaLine = ""
	tm.fileLabel = ""

	total := transferred + failed
	tm.summaryLine = fmt.Sprintf("Transferred %d/%d files", transferred, total)

	if failed > 0 {
		tm.summaryLine += fmt.Sprintf(", %d failed", failed)
		if len(failedFiles) > 0 {
			// Show up to 3 failed file names
			limit := len(failedFiles)
			if limit > 3 {
				limit = 3
			}
			names := strings.Join(failedFiles[:limit], ", ")
			if len(failedFiles) > 3 {
				names += fmt.Sprintf(" (+%d more)", len(failedFiles)-3)
			}
			tm.summaryLine += "\n" + names
		}
	}

	tm.SetTitle(" Transfer Complete ")
}

// Hide dismisses the modal and calls the onDismiss callback if set.
func (tm *TransferModal) Hide() {
	tm.visible = false
	if tm.onDismiss != nil {
		tm.onDismiss()
	}
}

// IsVisible returns whether the modal is currently displayed.
func (tm *TransferModal) IsVisible() bool {
	return tm.visible
}

// SetDismissCallback sets the function called when the modal is hidden.
func (tm *TransferModal) SetDismissCallback(fn func()) {
	tm.onDismiss = fn
}

// HandleKey processes keyboard input for the transfer modal.
// Behavior depends on the current mode:
//
//	modeProgress: Esc enters cancel confirm; other keys pass through
//	modeCancelConfirm: Esc/y/Enter confirm cancel, n resumes transfer; other keys consumed
//	modeSummary: any key dismisses
//	modeConflictDialog: Plan 02 implementation
func (tm *TransferModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !tm.visible {
		return event
	}

	switch tm.mode {
	case modeConflictDialog:
		switch event.Rune() {
		case 'o':
			if tm.conflictActionCh != nil {
				tm.conflictActionCh <- domain.ConflictOverwrite
			}
			tm.mode = modeProgress
			return nil
		case 's':
			if tm.conflictActionCh != nil {
				tm.conflictActionCh <- domain.ConflictSkip
			}
			tm.mode = modeProgress
			return nil
		case 'r':
			if tm.conflictActionCh != nil {
				tm.conflictActionCh <- domain.ConflictRename
			}
			tm.mode = modeProgress
			return nil
		}
		return nil // consume all other keys in conflict dialog mode

	case modeCancelConfirm:
		switch event.Key() {
		case tcell.KeyEscape:
			// D-03: second Esc confirms cancel (same as y/Enter)
			tm.cancelConfirmed = true
			tm.onDismiss() // triggers cancel flow in FileBrowser
			return nil
		case tcell.KeyEnter:
			tm.cancelConfirmed = true
			tm.onDismiss() // triggers cancel flow in FileBrowser
			return nil
		}
		switch event.Rune() {
		case 'y':
			tm.cancelConfirmed = true
			tm.onDismiss()
			return nil
		case 'n':
			tm.ResumeProgress()
			return nil
		}
		return nil // consume all other keys in cancel confirm mode

	case modeSummary:
		tm.Hide()
		return nil

	case modeProgress:
		switch event.Key() {
		case tcell.KeyEscape:
			tm.ShowCancelConfirm()
			return nil
		}
		return event // pass through other keys
	}

	return event
}

// Update refreshes the modal with new transfer progress data.
// The caller is responsible for wrapping calls in app.QueueUpdateDraw().
// No-op if not in progress mode.
func (tm *TransferModal) Update(p domain.TransferProgress) {
	if tm.mode != modeProgress {
		return
	}

	// Update progress bar
	tm.bar.SetProgress(p.BytesDone, p.BytesTotal)

	// Calculate speed using sliding window
	speed := tm.calculateSpeed(p.BytesDone)

	// Update percentage text
	pct := float64(0)
	if p.BytesTotal > 0 {
		pct = float64(p.BytesDone) / float64(p.BytesTotal) * 100
		if pct > 100 {
			pct = 100
		}
	}

	// Build info line: percentage + speed
	tm.infoLine = fmt.Sprintf("%.0f%%", pct)
	if speed > 0 {
		tm.infoLine += "  " + formatSpeed(speed)
	}

	// Calculate and update ETA
	if speed > 0 && p.BytesTotal > 0 {
		remaining := float64(p.BytesTotal-p.BytesDone) / speed
		tm.etaLine = "ETA: " + formatETA(remaining)
	} else {
		tm.etaLine = ""
	}

	// Handle completion
	if p.Done {
		tm.bar.SetColor(completedColor)
		tm.etaLine = "Complete"
	}

	// Handle failure
	if p.Failed {
		tm.infoLine = "Transfer failed"
		tm.etaLine = ""
	}

	// Update title for multi-file transfers
	if p.FileTotal > 0 {
		title := tm.GetTitle()
		if strings.Contains(title, "Uploading") || strings.Contains(title, "Downloading") {
			// Extract direction from current title
			direction := "Transfer"
			if strings.Contains(title, "Uploading") {
				direction = "Uploading"
			} else if strings.Contains(title, "Downloading") {
				direction = "Downloading"
			}
			newTitle := fmt.Sprintf(" %s (file %d/%d) ", direction, p.FileIndex, p.FileTotal)
			tm.SetTitle(newTitle)
		}
	}
}

// calculateSpeed computes transfer speed using a sliding window of recent samples.
// Returns bytes per second, or 0 if insufficient data.
func (tm *TransferModal) calculateSpeed(bytesDone int64) float64 {
	now := time.Now()
	sample := speedSample{time: now, bytes: bytesDone}

	tm.speedSamples = append(tm.speedSamples, sample)
	if len(tm.speedSamples) > maxSpeedSamples {
		tm.speedSamples = tm.speedSamples[len(tm.speedSamples)-maxSpeedSamples:]
	}

	if len(tm.speedSamples) < 2 {
		return 0
	}

	oldest := tm.speedSamples[0]
	latest := tm.speedSamples[len(tm.speedSamples)-1]
	elapsed := latest.time.Sub(oldest.time).Seconds()
	if elapsed <= 0 {
		return 0
	}

	return float64(latest.bytes-oldest.bytes) / elapsed
}
