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

	"github.com/gdamore/tcell/v2"
)

const (
	// defaultBarWidth is the default character width of the progress bar.
	defaultBarWidth = 30

	// fillChar is the Unicode character used for the filled portion of the bar.
	fillChar rune = '\u2588' // █

	// emptyChar is the Unicode character used for the empty portion of the bar.
	emptyChar rune = '\u2591' // ░
)

// ProgressBar renders an ASCII progress bar using Unicode block characters.
// It tracks current/total progress and produces a string representation
// suitable for display in tview.TextView or manual screen drawing.
type ProgressBar struct {
	current   int64
	total     int64
	width     int
	fillRune  rune
	emptyRune rune
	color     tcell.Color
}

// NewProgressBar creates a new ProgressBar with sensible defaults.
// Default width is 30 characters, fill='█', empty='░', color=Color248.
func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		width:     defaultBarWidth,
		fillRune:  fillChar,
		emptyRune: emptyChar,
		color:     tcell.Color248,
	}
}

// SetProgress updates the current and total byte counts.
func (pb *ProgressBar) SetProgress(current, total int64) {
	pb.current = current
	pb.total = total
}

// SetColor changes the color used when the bar is drawn in a styled context.
func (pb *ProgressBar) SetColor(color tcell.Color) {
	pb.color = color
}

// SetWidth sets the explicit character width of the bar.
// A width of 0 or negative causes the default (30) to be used.
func (pb *ProgressBar) SetWidth(width int) {
	if width <= 0 {
		width = defaultBarWidth
	}
	pb.width = width
}

// String renders the progress bar as a string of Unicode block characters.
// If total is 0, returns an empty string (no progress to show).
// Clamps fillCount to [0, width] to avoid edge-case overflows.
func (pb *ProgressBar) String() string {
	if pb.total == 0 {
		return ""
	}

	pct := float64(pb.current) / float64(pb.total)
	if pct > 1.0 {
		pct = 1.0
	}

	width := pb.width
	if width <= 0 {
		width = defaultBarWidth
	}

	fillCount := int(pct * float64(width))
	if fillCount > width {
		fillCount = width
	}
	if fillCount < 0 {
		fillCount = 0
	}

	emptyCount := width - fillCount
	return strings.Repeat(string(pb.fillRune), fillCount) + strings.Repeat(string(pb.emptyRune), emptyCount)
}

// formatSpeed converts a bytes-per-second value to a human-readable string.
// Returns "X.X KB/s" for values under 1 MB/s, "X.X MB/s" otherwise.
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1024/1024)
	}
	return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
}

// formatETA converts seconds remaining to a compact "Xm Ys" string.
// Returns empty string for zero, negative, or values exceeding 24 hours.
func formatETA(seconds float64) string {
	if seconds <= 0 || seconds > 86400 {
		return ""
	}
	totalSecs := int(seconds)
	mins := totalSecs / 60
	secs := totalSecs % 60
	return fmt.Sprintf("%dm %ds", mins, secs)
}
