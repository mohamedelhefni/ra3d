package tui

import (
	"fmt"
	"strings"
)

const clearScreen = "\033[H\033[J" // Clears the screen and moves the cursor to the top-left
const moveUp = "\033[A"            // Moves cursor up one line

type TorrentTui struct {
	DisplayName string
	Peers       int
	Percentage  int
}

func (tt *TorrentTui) Listen(ch chan TorrentTui) {
	for update := range ch {
		tt.Draw(update)
	}
}

func (*TorrentTui) Draw(tt TorrentTui) error {
	fmt.Print(clearScreen)

	fmt.Println("Name: ", tt.DisplayName)
	fmt.Println("Peers: ", tt.Peers)

	barWidth := 50
	progress := tt.Percentage
	if progress > 100 {
		progress = 100
	} else if progress < 0 {
		progress = 0
	}

	filled := int(float64(barWidth) * (float64(progress) / 100.0))
	empty := barWidth - filled

	progressBar := fmt.Sprintf("[%s%s] %d%%", strings.Repeat("=", filled), strings.Repeat(" ", empty), progress)
	fmt.Println(progressBar)

	return nil
}
