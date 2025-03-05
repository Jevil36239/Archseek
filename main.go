package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"archseek/banner"
	"archseek/Module"
)

var (
	cyan = color.New(color.FgHiBlue)
	red  = color.New(color.FgHiBlack)
)

func main() {
	banner.Print(banner.DefaultConfig())


	fmt.Print("\nEnter domain (e.g., example.com) or press Enter to load from file: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	domainInput := strings.TrimSpace(scanner.Text())

	if domainInput == "" {
		fmt.Print("Enter file name containing domains: ")
		scanner.Scan()
		fileName := strings.TrimSpace(scanner.Text())

		file, err := os.Open(fileName)
		if err != nil {
			red.Print("[ERROR] ")
			fmt.Printf("File %s not found\n", fileName)
			return
		}
		defer file.Close()

		var domains []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				domains = append(domains, line)
			}
		}

		cyan.Print("[INFO] ")
		red.Printf("%d ", len(domains))
		fmt.Printf("domains in %s\n", fileName)

		module.ProcessDomains(domains)
	} else {
		module.ProcessDomains([]string{domainInput})
	}
}
