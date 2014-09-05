package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/christophwitzko/go-curl"
	"github.com/olekukonko/ts"
	"github.com/visionmedia/go-spin"
	"github.com/wsxiaoys/terminal"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const Version string = "dwnldr v0.0.1"

var (
	maxSpeed        int64
	tHeight, tWidth int
)

type DownloadStat struct {
	stat     *curl.IoCopyStat
	sp       *spin.Spinner
	url      string
	out      string
	shortout string
	id       int
	err      error
}

func PrintUsage() {
	fmt.Println(`
Usage: dwnldr [options] <urls>...
  Examples:
    dwnldr http://de.edis.at/100MB.test http://at.edis.at/100MB.test
    dwnldr -s 1M http://de.edis.at/100MB.test
  Options:
    -h      Show this screen.
    -v      Show version.
    -o=<s>  Set output filename.
    -d=<s>  Set output root directory.
    -p      Set parallel download mode.
    -s=<b>  Set speed limit in Bytes/s [default: 0 (no limit)].
  `)
}

func parseMetric(s string) (float64, error) {
	s = strings.ToUpper(s)
	units_metric := []string{"K", "M", "G", "T", "P"}
	scale_metric := float64(1024)
	if len(s) < 1 {
		return 0, errors.New("string too short")
	}
	idx := float64(1)
	for i, t := range units_metric {
		if strings.HasSuffix(s, t) {
			idx = math.Pow(scale_metric, float64(i+1))
			s = s[:len(s)-len(t)]
			break
		}
	}
	pf, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	pf *= idx
	return pf, nil
}

func mapContains(fm map[string]string, fs string) bool {
	for _, s := range fm {
		if s == fs {
			return true
		}
	}
	return false
}

func strPadding(instr string, padlen int) string {
	pdiff := padlen - len(instr)
	if pdiff <= 0 {
		return instr
	}
	return strings.Repeat(" ", pdiff) + instr
}

func strLimitAndPad(instr string, maxlen int) string {
	if maxlen < 3 {
		return instr
	}
	if len(instr) <= maxlen {
		return strPadding(instr, maxlen)
	}
	return instr[:maxlen-1] + "…"
}

func genName(fu map[string]string, fn string) string {
	baseFn := fn
	for i := 1; mapContains(fu, fn); i++ {
		fn = fmt.Sprintf("%s-%d", baseFn, i)
		fmt.Println(fn)
	}
	return fn
}

func getBar(maxlen int, per float64) string {
	curser := ">"
	barchar := "="
	emptychar := " "
	bar := ""
	maxlen -= 1
	pos := int(float64(maxlen) * per)
	if pos > 0 {
		bar += strings.Repeat(barchar, pos) + curser
	} else {
		bar += emptychar
	}

	bar += strings.Repeat(emptychar, maxlen-pos)
	return bar
}

func writeLine(l int, txt string) {
	terminal.Stdout.Move(l, 0).ClearLine().Print(txt)
}

func averageFloat64(vals map[int]float64) float64 {
	sum := float64(0)
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func downloadFromUrl(url string, out string, l int, done chan int, stat chan DownloadStat) string {
	s := spin.New()
	_, ifn := path.Split(out)
	cb := func(st curl.IoCopyStat) error {
		stat <- DownloadStat{&st, s, url, out, ifn, l, nil}
		return nil
	}
	err, _ := curl.File(url, out, cb, "maxspeed=", maxSpeed, "cbinterval=", 0.2)
	if err != nil {
		stat <- DownloadStat{&curl.IoCopyStat{}, s, url, out, ifn, l, err}
	}
	done <- l
	return out
}

func downloadAllFiles(fu map[string]string, prl bool) {
	if len(fu) < 1 {
		return
	}
	mstrar := make([][]string, len(fu))
	i := 0
	for u, f := range fu {
		mstrar[i] = []string{u, f}
		i++
	}
	dch := make(chan int)
	statch := make(chan DownloadStat)
	offset := 1
	count := 0
	lenmstrar := len(mstrar)
	gsp := spin.New()
	gsp.Set(spin.Spin5)
	statprog := make(map[int]float64, lenmstrar)
	if prl {
		for i, v := range mstrar {
			go downloadFromUrl(v[0], v[1], i+offset, dch, statch)
		}
	} else {
		go downloadFromUrl(mstrar[count][0], mstrar[count][1], count+offset, dch, statch)
	}
dfl:
	for {
		select {
		case <-dch:
			count++
			if count >= lenmstrar {
				writeLine(lenmstrar+offset, fmt.Sprintf("G [%s] 100.0%% | %d/%d File(s)\n", getBar(28, 1), count, lenmstrar))
				break dfl
			} else if !prl {
				go downloadFromUrl(mstrar[count][0], mstrar[count][1], count+offset, dch, statch)
			}
		case st := <-statch:
			if st.err != nil {
				statprog[st.id] = float64(1)
				writeLine(st.id, fmt.Sprintf("E %s: %s", strLimitAndPad(st.url, 20), st.err))
				continue dfl
			}
			statprog[st.id] = st.stat.Per
			avg := averageFloat64(statprog)
			writeLine(lenmstrar+offset, fmt.Sprintf("%s [%s] %s | %d/%d File(s)", gsp.Next(), getBar(28, avg), strPadding(curl.PrettyPer(avg), 6), count, lenmstrar))
			if st.stat.Per >= 1.0 {
				writeLine(st.id, fmt.Sprintf("D [%s] [%s]: %s", getBar(28, 1), strPadding(st.stat.Durstr, 6), strLimitAndPad(st.shortout, 15)))
				continue dfl
			}
			writeLine(st.id, fmt.Sprintf("%s [%s] %s | %s | %s", st.sp.Next(), getBar(28, st.stat.Per), strPadding(st.stat.Perstr, 6), strPadding(st.stat.Speedstr, 10), strLimitAndPad(st.shortout, 15)))
		}
	}
}

func main() {
	flag.Usage = PrintUsage

	help := flag.Bool("h", false, "help")
	version := flag.Bool("v", false, "version")
	parallel := flag.Bool("p", false, "parallel")
	rawMaxSpeed := flag.String("s", "0", "speed")
	defOutputFile := flag.String("o", "", "output")
	defOutputDir := flag.String("d", "./", "root")

	flag.Parse()

	tSize, err := ts.GetSize()
	if err != nil {
		fmt.Println("Cannot get terminal width:", err)
		return
	}
	tWidth = tSize.Col()
	tHeight = tSize.Row()

	if *help {
		flag.Usage()
		return
	}
	if *version {
		fmt.Println("Version: " + Version)
		return
	}
	mspeed, err := parseMetric(*rawMaxSpeed)
	if err != nil {
		fmt.Println("Wrong format:", *rawMaxSpeed)
		flag.Usage()
		return
	}
	maxSpeed = int64(mspeed)
	fileurls := map[string]string{}
	for _, u := range flag.Args() {
		pu, err := url.Parse(u)
		if err != nil {
			fmt.Println("Wrong URL:", u)
			flag.Usage()
			return
		}
		pfn := "index"
		if _, psp := path.Split(pu.Path); len(psp) > 1 {
			pfn = psp
		}
		if len(*defOutputFile) > 0 {
			pfn = *defOutputFile
		}
		fileurls[u] = path.Join(*defOutputDir, genName(fileurls, pfn))
	}
	if len(fileurls) < 1 {
		fmt.Println("URL(s) missing.")
		flag.Usage()
		return
	}
	terminal.Stdout.Clear()
	downloadAllFiles(fileurls, *parallel)
}
