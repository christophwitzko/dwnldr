package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/christophwitzko/go-curl"
	"github.com/visionmedia/go-spin"
	"github.com/wsxiaoys/terminal"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const Version string = "dwnldr v0.0.1"

var maxSpeed int64

func PrintUsage() {
	fmt.Println(`
Usage: dwnldr [options] <urls>...
  Examples:
    dwnldr http://de.edis.at/100MB.test
    dwnldr -s 1M http://de.edis.at/100MB.test
  Options:
    -h      Show this screen.
    -v      Show version.
    -o      Set output filename.
    -d      Set output root directory.
    -p      Set parallel download mode.
    -s=<b>  Speed in Bytes/s [default: 0 (no limit)].
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

func genName(fu map[string]string, fn string) string {
	baseFn := fn
	for i := 1; mapContains(fu, fn); i++ {
		fn = fmt.Sprintf("%s-%d", baseFn, i)
		fmt.Println(fn)
	}
	return fn
}

func writeLine(l int, txt string) {
	terminal.Stdout.Move(l, 0).ClearLine().Print(txt)
}

func downloadFromUrl(url string, out string, l int, done chan int) string {
	s := spin.New()
	_, ifn := path.Split(out)
	ldstr := ""
	cb := func(st curl.IoCopyStat) error {
		writeLine(l, fmt.Sprintf(" %s [%s]: %s %s", s.Next(), ifn, st.Perstr, st.Speedstr))
		ldstr = st.Durstr
		return nil
	}
	curl.File(url, out, cb, "maxspeed=", maxSpeed, "cbinterval=", 0.1)
	writeLine(l, fmt.Sprintf("   [%s] %s: done, duration: %s", ifn, ldstr))
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
	offset := 1
	count := 0
	if prl {
		for i, v := range mstrar {
			go downloadFromUrl(v[0], v[1], i+offset, dch)
		}
	} else {
		go downloadFromUrl(mstrar[count][0], mstrar[count][1], count+offset, dch)
	}
dfl:
	for {
		select {
		case ln := <-dch:
			writeLine(ln, "Done: "+strconv.Itoa(count+1)+"\n")
			count++
			if count >= len(mstrar) {
				break dfl
			} else if !prl {
				go downloadFromUrl(mstrar[count][0], mstrar[count][1], count+offset, dch)
			}
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
		if strings.Contains(pu.Path, "/") && len(pu.Path) > 1 {
			sss := strings.Split(pu.Path, "/")
			pfn = sss[len(sss)-1]
		}
		if len(*defOutputFile) > 0 {
			pfn = *defOutputFile
		}
		fileurls[u] = path.Join(*defOutputDir, genName(fileurls, pfn))
	}
	terminal.Stdout.Clear()
	downloadAllFiles(fileurls, *parallel)
}
