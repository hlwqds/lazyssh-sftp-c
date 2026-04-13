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
)

// TransferModal is a full-screen overlay component that displays file transfer progress.
// It renders a progress bar, file name, transfer speed, ETA, and supports
// directory transfer summaries. It embeds *tview.Box for background rendering
// and implements manual Draw for precise layout control.
//
// All UI updates must go through app.QueueUpdateDraw() for thread safety.
type TransferModal struct {
	*tview.Box
	app *tview.Application
	bar *ProgressBar

	// display state
	fileLabel   string // "Uploading: filename.txt"
	infoLine    string // "67%  2.3 MB/s"
	etaLine     string // "ETA: 0m 12s"
	summaryLine string // "Transferred 8/10 files, 2 failed"
	showSummary bool   // true when showing directory transfer summary

	// speed tracking
	speedSamples []speedSample

	// modal state
	visible   bool
	onDismiss func()
}

// NewTransferModal creates a new TransferModal overlay component.
func NewTransferModal(app *tview.Application) *TransferModal {
	tm := &TransferModal{
		Box:           tview.NewBox().SetBorderPadding(2, 2, 5, 5),
		app:           app,
		bar:           NewProgressBar(),
		visible:       false,
		speedSamples:  make([]speedSample, 0, maxSpeedSamples),
	}
	tm.SetBorder(true).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250).
		SetBackgroundColor(tcell.Color232)
	return tm
}

// Draw renders the transfer modal to the screen.
// Layout centers the progress content within the modal box:
//
//	+-- Transfer Title ------------------+
//	|                                    |
//	|     Uploading: filename.txt        |
//	|                                    |
//	|   ████████████░░░░░  67%           |
//	|                2.3 MB/s            |
//	|                ETA: 0m 12s         |
//	|                                    |
//	+------------------------------------+
func (tm *TransferModal) Draw(screen tcell.Screen) {
	if !tm.visible {
		return
	}
	tm.Box.DrawForSubclass(screen, tm)

	x, y, width, height := tm.GetInnerRect()

	// Row 1: file name label (or summary line)
	row1 := y + 1
	if tm.showSummary {
		tview.Print(screen, tm.summaryLine, x, row1, width, tview.AlignCenter, tcell.Color248)
	} else {
		tview.Print(screen, tm.fileLabel, x, row1, width, tview.AlignCenter, tcell.Color255)
	}

	// Skip bar/info/eta when showing summary
	if tm.showSummary {
		tm.drawSummaryFooter(screen, x, y, width, height)
		return
	}

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

// drawSummaryFooter renders the bottom portion of the summary view
// with a "Press any key to close" hint.
func (tm *TransferModal) drawSummaryFooter(screen tcell.Screen, x, y, width, height int) {
	hint := "Press any key to close"
	footerRow := y + height - 2
	tview.Print(screen, hint, x, footerRow, width, tview.AlignCenter, tcell.Color245)
}

// Show displays the modal as a full-screen overlay.
// direction should be "Uploading" or "Downloading".
func (tm *TransferModal) Show(direction, filename string) {
	tm.visible = true
	tm.showSummary = false

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

// Update refreshes the modal with new transfer progress data.
// The caller is responsible for wrapping calls in app.QueueUpdateDraw().
func (tm *TransferModal) Update(p domain.TransferProgress) {
	if tm.showSummary {
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

// ShowSummary displays a directory transfer summary instead of per-file progress.
func (tm *TransferModal) ShowSummary(transferred, failed int, failedFiles []string) {
	tm.showSummary = true

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
// Esc always dismisses. Any key in summary mode dismisses.
func (tm *TransferModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !tm.visible {
		return event
	}

	// Esc: always dismiss
	//nolint:exhaustive // We only handle Esc and pass through others
	switch event.Key() {
	case tcell.KeyEscape:
		tm.Hide()
		return nil
	}

	// In summary mode, any key dismisses
	if tm.showSummary {
		tm.Hide()
		return nil
	}

	return event
}
