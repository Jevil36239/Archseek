package banner

import (
	"fmt"
	"github.com/fatih/color"
	"strings"
	"gopkg.in/ini.v1"
	"time"
)

type BannerConfig struct {
	Text           string
	TextColor      *color.Color
	BackgroundColor *color.Color
	Author         string
	ShowDate       bool
	DateFormat     string
}

func DefaultConfig() *BannerConfig {
	return &BannerConfig{
		Text: `
╭               		  ╮
              ⠠⠁⠗⠉⠓⠎⠑⠑⠅ ArchSeek ⠠⠁⠗⠉⠓⠎⠑⠑⠅ 
╰               		  ╯`,
		TextColor:      color.New(color.FgHiGreen),
		BackgroundColor: color.New(color.FgHiBlack),
		Author:         "Jevil36239",
		ShowDate:       true,
		DateFormat:     "2006-01-02 15:04:05",
	}
}

func centerText(text string, width int) string {
	lines := strings.Split(text, "\n")
	centeredLines := make([]string, len(lines))
	
	for i, line := range lines {
		lineLen := len(line)
		if lineLen < width {
			padding := (width - lineLen) / 2
			centeredLines[i] = strings.Repeat(" ", padding) + line
		} else {
			centeredLines[i] = line
		}
	}
	
	return strings.Join(centeredLines, "\n")
}

func Print(config *BannerConfig) {
	if config == nil {
		config = DefaultConfig()
	}

	centeredText := centerText(config.Text, 50)
	lines := strings.Split(centeredText, "\n")
	backgroundColorFunc := config.BackgroundColor.SprintFunc()

	for _, line := range lines {
		fmt.Println(config.TextColor.Sprint(backgroundColorFunc(line)))
	}

// Display File Extensions
cfg, err := ini.Load("settings.ini")
if err == nil {
    extensions := cfg.Section("FileExtensions").Key("Extensions").String()
    extList := strings.Split(strings.ReplaceAll(extensions, "\\.", "."), "|")
    
    // Header dengan garis pemisah
    separator := centerText("──────────────────────────────────", 50)
    fmt.Println(color.New(color.FgHiBlack).Sprint(separator))
    fmt.Println(color.New(color.FgHiCyan).Sprint(centerText("Supported File Extensions", 50)))
    
    // Format ekstensi dalam grid terpusat
    var extLine strings.Builder
    for i, ext := range extList {
        extLine.WriteString(fmt.Sprintf("• %-8s", ext))
        
        // Membuat baris baru setiap 4 item
        if (i+1)%4 == 0 || i == len(extList)-1 {
            fmt.Println(color.New(color.FgHiWhite).Sprint(centerText(extLine.String(), 50)))
            extLine.Reset()
        }
    }
    
    fmt.Println(color.New(color.FgHiBlack).Sprint(separator))
}

	if config.Author != "" {
		authorText := "Created by: " + config.Author
		centeredAuthor := centerText(authorText, 50)
		fmt.Println(color.New(color.FgHiCyan).Sprint(centeredAuthor))
	}

	if config.ShowDate {
		currentTime := time.Now().Format(config.DateFormat)
		dateText := "Date: " + currentTime
		centeredDate := centerText(dateText, 50)
		fmt.Println(color.New(color.FgHiCyan).Sprint(centeredDate))
	}
}