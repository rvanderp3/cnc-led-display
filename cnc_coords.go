package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAddress  = "192.168.0.41:23"
	defaultWLED     = "http://192.168.0.52"
	timeout         = 5 * time.Second
	matrixSize      = 32
	maxRetryDelay   = 30 * time.Second
)

// wledAddr is set from the -wled-addr flag after flag.Parse.
var wledAddr string

// prevLitPixels tracks which LED indices were lit in the last frame so they
// can be explicitly zeroed when they are no longer needed.
var prevLitPixels map[int]string

func main() {
	addr := flag.String("addr", defaultAddress, "CNC address (host:port)")
	wledAddrFlag := flag.String("wled-addr", defaultWLED, "WLED controller base URL")
	clearOnly := flag.Bool("clear", false, "Just clear the display and exit")
	flag.Parse()

	wledAddr = *wledAddrFlag

	if *clearOnly {
		fmt.Println("Clearing WLED display...")
		clearWLED()
		fmt.Println("Done.")
		return
	}

	fmt.Println("Connecting to CNC and WLED display...")
	fmt.Println("Press Ctrl+C to exit")

	clearWLED()

	retryDelay := time.Second
	for {
		conn, err := net.DialTimeout("tcp", *addr, timeout)
		if err != nil {
			log.Printf("Error connecting to CNC: %v (retrying in %s)", err, retryDelay)
			time.Sleep(retryDelay)
			if retryDelay < maxRetryDelay {
				retryDelay *= 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
			}
			continue
		}
		retryDelay = time.Second

		conn.SetDeadline(time.Now().Add(timeout))

		_, err = conn.Write([]byte("?\n"))
		if err != nil {
			log.Printf("Error sending status request: %v", err)
			conn.Close()
			continue
		}

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "WPos:") || strings.Contains(line, "MPos:") {
				parseAndPrintStatus(line)
				sendToWLED(line)
				break
			}
		}

		conn.Close()
		time.Sleep(500 * time.Millisecond)
	}
}

func parseAndPrintStatus(line string) {
	line = strings.TrimLeft(line, "<><>")
	parts := strings.Split(line, "|")
	if len(parts) == 0 {
		return
	}

	status := parts[0]
	fmt.Printf("\n--- CNC Status ---\n")
	fmt.Printf("State:      %s\n", status)

	for _, part := range parts[1:] {
		if strings.Contains(part, "WPos:") || strings.Contains(part, "MPos:") {
			subParts := strings.Split(part, ":")
			if len(subParts) > 1 {
				fmt.Printf("Position:   %s\n", subParts[1])
			}
		} else if strings.HasPrefix(part, "Bf:") {
			fmt.Printf("Buffer:     %s\n", strings.TrimPrefix(part, "Bf:"))
		} else if strings.HasPrefix(part, "FS:") {
			fmt.Printf("Feed/Speed: %s\n", strings.TrimPrefix(part, "FS:"))
		} else if strings.HasPrefix(part, "Ov:") {
			fmt.Printf("Overlay:    %s\n", strings.TrimPrefix(part, "Ov:"))
		}
	}
	fmt.Printf("------------------\n")
}

func parseState(line string) string {
	line = strings.TrimLeft(line, "<>")
	parts := strings.Split(line, "|")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func sendToWLED(line string) {
	coords := parseCoords(line)
	if coords == nil {
		return
	}

	statusColor := "FF0000" // red = busy/unknown
	if parseState(line) == "Idle" {
		statusColor = "00FF00" // green = idle
	}

	now := time.Now()
	timeStr := now.Format("15:04")

	lines := []struct {
		text  string
		color string
	}{
		{fmt.Sprintf("X%.1f", coords[0]), "FF0000"},
		{fmt.Sprintf("Y%.1f", coords[1]), "00FF00"},
		{fmt.Sprintf("Z%.1f", coords[2]), "0000FF"},
		{timeStr, "FFFFFF"},
	}

	sendMultiLineToWLED(lines, statusColor)
}

func parseCoords(line string) []float64 {
	line = strings.TrimLeft(line, "<><>")
	parts := strings.Split(line, "|")
	if len(parts) == 0 {
		return nil
	}

	var position string
	for _, part := range parts[1:] {
		if strings.Contains(part, "WPos:") || strings.Contains(part, "MPos:") {
			subParts := strings.Split(part, ":")
			if len(subParts) > 1 {
				position = subParts[1]
				break
			}
		}
	}

	if position == "" {
		log.Printf("parseCoords: no position field in status line: %q", line)
		return nil
	}

	coords := strings.Split(position, ",")
	if len(coords) < 3 {
		log.Printf("parseCoords: expected 3 coordinates, got %d in %q", len(coords), position)
		return nil
	}

	var x, y, z float64
	if _, err := fmt.Sscanf(coords[0], "%f", &x); err != nil {
		log.Printf("parseCoords: bad X value %q: %v", coords[0], err)
		return nil
	}
	if _, err := fmt.Sscanf(coords[1], "%f", &y); err != nil {
		log.Printf("parseCoords: bad Y value %q: %v", coords[1], err)
		return nil
	}
	if _, err := fmt.Sscanf(coords[2], "%f", &z); err != nil {
		log.Printf("parseCoords: bad Z value %q: %v", coords[2], err)
		return nil
	}

	return []float64{x, y, z}
}

var digitPatterns = [10][5]int{
	{0x7, 0x5, 0x5, 0x5, 0x7},
	{0x2, 0x6, 0x2, 0x2, 0x7},
	{0x7, 0x1, 0x7, 0x4, 0x7},
	{0x7, 0x1, 0x7, 0x1, 0x7},
	{0x5, 0x5, 0x7, 0x1, 0x1},
	{0x7, 0x4, 0x7, 0x1, 0x7},
	{0x7, 0x4, 0x7, 0x5, 0x7},
	{0x7, 0x1, 0x2, 0x2, 0x2},
	{0x7, 0x5, 0x7, 0x5, 0x7},
	{0x7, 0x5, 0x7, 0x1, 0x7},
}

func drawFrame(leds map[int]string, color string) {
	for i := 0; i < matrixSize; i++ {
		setLED(leds, i, 0, color)
		setLED(leds, i, 1, color)
		setLED(leds, i, 30, color)
		setLED(leds, i, 31, color)
		setLED(leds, 0, i, color)
		setLED(leds, 1, i, color)
		setLED(leds, 30, i, color)
		setLED(leds, 31, i, color)
	}
}

func sendMultiLineToWLED(lines []struct {
	text  string
	color string
}, statusColor string) {
	fmt.Printf("[sendMultiLineToWLED] Drawing lines: %v\n", lines)

	// Only collect lit pixels; background is cleared via "col":[[0,0,0]] in the
	// segment payload, keeping the JSON well under WLED's ~4KB parse limit.
	litPixels := make(map[int]string)

	for lineIdx, line := range lines {
		startY := lineIdx*7 + 3
		xCursor := 3

		for _, ch := range line.text {
			switch {
			case ch == '-':
				drawMinusLit(litPixels, xCursor+1, startY, line.color)
				xCursor += 5 // 1px left margin + 3px glyph + 1px extra gap before digit
			case ch >= '0' && ch <= '9':
				drawDigitLit(litPixels, int(ch-'0'), xCursor, startY, line.color)
				xCursor += 4
			case ch == '.':
				drawDotLit(litPixels, xCursor, startY, line.color)
				xCursor += 2
			case ch == 'X':
				drawXLit(litPixels, xCursor, startY, line.color)
				xCursor += 5
			case ch == 'Y':
				drawYLit(litPixels, xCursor, startY, line.color)
				xCursor += 5
			case ch == 'Z':
				drawZLit(litPixels, xCursor, startY, line.color)
				xCursor += 5
			case ch == ':':
				drawColonLit(litPixels, xCursor+1, startY+1, line.color)
				xCursor += 4
			}
		}
	}
	drawFrame(litPixels, statusColor)
	printASCIIArt(litPixels)

	var buf bytes.Buffer
	buf.WriteString(`{"seg":{"on":true,"bri":255,"i":[`)
	first := true
	// Zero out any pixel that was lit last frame but isn't in this frame.
	for idx := range prevLitPixels {
		if _, stillLit := litPixels[idx]; !stillLit {
			if !first {
				buf.WriteString(",")
			}
			first = false
			fmt.Fprintf(&buf, "%d,\"000000\"", idx)
		}
	}
	for ledIdx, ledColor := range litPixels {
		if !first {
			buf.WriteString(",")
		}
		first = false
		fmt.Fprintf(&buf, "%d,\"%s\"", ledIdx, ledColor)
	}
	buf.WriteString("]}}")
	prevLitPixels = litPixels

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", wledAddr+"/json", &buf)
	if err != nil {
		log.Printf("sendMultiLineToWLED: failed to build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("sendMultiLineToWLED: HTTP POST failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("sendMultiLineToWLED: WLED returned %s: %s", resp.Status, body)
	}
}

func setLED(leds map[int]string, x, y int, color string) {
	if x >= 0 && x < matrixSize && y >= 0 && y < matrixSize {
		leds[y*matrixSize+x] = color
	}
}

func printASCIIArt(leds map[int]string) {
	fmt.Println("+" + strings.Repeat("-", matrixSize) + "+")
	for row := 0; row < matrixSize; row++ {
		fmt.Print("|")
		for col := 0; col < matrixSize; col++ {
			if color, ok := leds[row*matrixSize+col]; ok && color != "000000" {
				fmt.Print("*")
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Println("|")
	}
	fmt.Println("+" + strings.Repeat("-", matrixSize) + "+")
}

func drawColonLit(leds map[int]string, x, y int, color string) {
	setLED(leds, x, y, color)
	setLED(leds, x, y+3, color)
}

func drawDigitLit(leds map[int]string, digit, x, y int, color string) {
	pattern := digitPatterns[digit]
	for row := 0; row < 5; row++ {
		for col := 0; col < 3; col++ {
			if (pattern[row]>>(2-col))&1 == 1 {
				setLED(leds, x+col, y+row, color)
			}
		}
	}
}

func drawDotLit(leds map[int]string, x, y int, color string) {
	setLED(leds, x, y+4, color)
}

func drawMinusLit(leds map[int]string, x, y int, color string) {
	for col := 0; col < 3; col++ {
		setLED(leds, x+col, y+2, color)
	}
}

func drawXLit(leds map[int]string, x, y int, color string) {
	for row := 0; row < 5; row++ {
		switch {
		case row < 2:
			setLED(leds, x+row, y+row, color)
			setLED(leds, x+2-row, y+row, color)
		case row == 2:
			setLED(leds, x+1, y+row, color)
		default:
			setLED(leds, x+row-3, y+row, color)
			setLED(leds, x+5-row, y+row, color)
		}
	}
}

func drawYLit(leds map[int]string, x, y int, color string) {
	for row := 0; row < 5; row++ {
		if row < 2 {
			setLED(leds, x+row, y+row, color)
			setLED(leds, x+2-row, y+row, color)
		} else {
			setLED(leds, x+1, y+row, color)
		}
	}
}

func drawZLit(leds map[int]string, x, y int, color string) {
	for col := 0; col < 3; col++ {
		setLED(leds, x+col, y, color)
		setLED(leds, x+col, y+4, color)
	}
	for row := 1; row < 4; row++ {
		setLED(leds, x+4-row, y+row, color)
	}
}

func clearWLED() {
	fmt.Println("[clearWLED] Clearing display...")
	payload := `{"seg":{"on":false}}`

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", wledAddr+"/json", bytes.NewBufferString(payload))
	if err != nil {
		log.Printf("clearWLED: failed to build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("clearWLED: HTTP POST failed: %v", err)
		return
	}
	defer resp.Body.Close()
}
