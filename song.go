package main

import (
    "os"
    "bufio"
    "strings"
    "bytes"
    "fmt"
    "path/filepath"
    "regexp"
)

//Structs used when parsing a song file
type Song struct {
    Title string
    Section string
    StanzaCount int
    SongNumber int
    Stanzas []Stanza
    BeforeComments []string
    AfterComments []string
    Transpose int
}

type Stanza struct {
    ShowNumber bool
    IsChorus bool
    Number int
    BeforeComments []string
    AfterComments []string
    Lines []Line
}

type Line struct {
    Text string
    Chords []Chord
}

type Chord struct {
    Text string
    Position int
}

func ParseSongFile(filename string, transpose int) (*Song, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var (
        lines []Line
        stanzas []Stanza
        stanza_count = 1
        is_chorus = false
        title = filepath.Base(filename)[0:len(filepath.Base(filename))-5]
        section = ""
        scanner = bufio.NewScanner(file)
    )

    //We need to handle /r only as Mac OS <= 9 uses this as end-of-line marker
    //This is based on bufio/scan.go ScanLines function
    split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
        if atEOF && len(data) == 0 {
            return 0, nil, nil
        }
        i := bytes.IndexByte(data, '\n');

        if i < 0 {
            i = bytes.IndexByte(data, '\r');
        }

        ind := 0
        if i > 0 && data[i-1] == '\r' {
            ind = -1
        }

        if i >= 0 {

            // We have a full newline-terminated line.
            return i + 1, data[0:i + ind], nil
        }
        // If we're at EOF, we have a final, non-terminated line. Return it.
        if atEOF {
            return len(data), data[0:len(data)+ind], nil
        }
        // Request more data.
        return 0, nil, nil
    }
    scanner.Split(split)

    stanza_before_comments := make([]string, 0)
    stanza_after_comments := make([]string, 0)
    song_before_comments := make([]string, 0)
    //song_after_comments := make([]string, 0)
    //
    chord_regex := regexp.MustCompile("\\[.*?\\]")
    
    for scanner.Scan() {
        line := scanner.Text()

        //is this a command
        if strings.HasPrefix(line, "{") {
            if (strings.HasPrefix(line, "{start_of_chorus}")) {
                is_chorus = true
            } else if (strings.HasPrefix(line,"{end_of_chorus}")) {

            } else if (strings.HasPrefix(line, "{title:")) {
                title = parseCommand(line)
            } else if (strings.HasPrefix(line, "{section:")) {
                section = parseCommand(line)
            } else if (strings.HasPrefix(line, "{comments:")) {
                if (len(stanzas) == 0) {
                    song_before_comments = append(song_before_comments, parseCommand(line))
                } else {
                    if (len(lines) > 0) {
                        stanza_after_comments = append(stanza_after_comments, parseCommand(line))
                    } else {
                        stanza_before_comments = append(stanza_before_comments, parseCommand(line))
                    }
                }
            }
        //blank line separates stanzas
        } else if len(line) == 0 {
            if len(lines) > 0 {
                stanzas = append(stanzas, *&Stanza{
                    Lines: lines,
                    Number: stanza_count,
                    IsChorus: is_chorus,
                    BeforeComments: stanza_before_comments,
                    AfterComments: stanza_after_comments})

                stanza_count++
                is_chorus = false
                lines = make([]Line, 0)
                stanza_before_comments = make([]string, 0)
                stanza_after_comments = make([]string, 0)

                if (is_chorus) {
                    stanza_count++
                }
            }
        } else {
            chords_pos := chord_regex.FindAllStringIndex(line, -1)
            chord_len := 0
            chords := make([]Chord, 0)

            for _, pos := range chords_pos {
                chord_text := line[pos[0]+1:pos[1]-1]
                chord_len += pos[1] - pos[0]
                position := pos[1] - chord_len

                chord_text = transposeKey(chord_text, transpose)

                chords = append(chords, Chord{Text: chord_text, Position: position})
            }

            //remove all chord markers
            line = chord_regex.ReplaceAllString(line, "")
            lines = append(lines, Line{Text: line, Chords: chords})
        }
    }

    //check for last stanza
    if len(lines) > 0 {
        stanzas = append(stanzas, *&Stanza{
            Lines: lines,
            Number: stanza_count,
            IsChorus: is_chorus})
    }
    
    return &Song{
        Title: title,
        Section: section,
        StanzaCount: 0,
        SongNumber: -1,
        Stanzas: stanzas,
        BeforeComments: song_before_comments,
        Transpose: transpose},
        nil
}

func parseCommand(command string) string {
    return strings.TrimSpace(command[strings.Index(command, ":")+1:strings.Index(command, "}")])
}

func (song Song) String() string {
    var buffer bytes.Buffer

    if len(song.Section) > 0 {
        buffer.WriteString(fmt.Sprintf("Section: %s\n", song.Section))
    }

    if len(song.BeforeComments) > 0 {
        buffer.WriteString(fmt.Sprintf("/%s/\n", song.BeforeComments))
    }

    for _,s := range song.Stanzas {
        if s.IsChorus {
            buffer.WriteString("---CHORUS--\n")
        }
        buffer.WriteString(fmt.Sprintf("STANZA: %d\n", s.Number))
        for _,l := range s.Lines {
            buffer.WriteString(fmt.Sprintf("%s\n", l.Text))
        }
        if s.IsChorus {
            buffer.WriteString(fmt.Sprintln("---END CHORUS--"))
        }

        buffer.WriteString("\n")
    }

    return buffer.String()
}

func (song Song) HasBeforeComments() bool {
    return len(song.BeforeComments) > 0
}

func (line Line) HasChords() bool {
    return len(line.Chords) > 0
}

func (stanza Stanza) HasChords() bool {
    for _,l := range stanza.Lines {
        if l.HasChords() {
            return true
        }
    }

    return false
}

func (line Line) PreChordText(chord Chord) string {
    //first, find the chord
    ind := -1
    for i,ch := range line.Chords {
        if ch == chord {
            ind = i
            break
        }
    }

    if ind < 0 {
        return ""
    }

    //We need the text from the previous chord up to this chord
    ind--

    //chord is the first chord
    if ind < 0 {
        return line.Text[0:chord.Position]
    }

    pos := line.Chords[ind].Position + len(line.Chords[ind].Text)

    return line.Text[pos:chord.Position]
}


var scales = map[string]int{
    "A": 0,
    "Bb": 1,
    "B": 2,
    "C": 3,
    "C#": 4,
    "D": 5,
    "D#": 6,
    "E": 7,
    "F": 8,
    "F#": 9,
    "G": 10,
    "G#": 11,
}

func transposeKey(key string, change int) string {
    if change == 0 {
        return key
    }

    //check first two letters for match
    var scale_ind = -1
    var ok = false
    if len(key) > 1 {
        scale_ind, ok = scales[key[0:2]]
        if !ok {
            scale_ind = -1
        }
    }

    //check for single key match
    if scale_ind < 0 && len(key) > 0 {
        scale_ind, ok = scales[key[0:1]]
        if !ok {
            scale_ind = -1
        }
    }

    if scale_ind < 0 {
        return key
    }

    scale_ind = (scale_ind + change) % (len(scales))
    for k := range scales {
        if scales[k] == scale_ind {
            return k
        }
    }

    return key
}
