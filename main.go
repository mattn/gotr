package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/fsnotify.v1"
)

const name = "gotr"

const version = "0.0.0"

var revision = "HEAD"

var (
	m sync.Mutex
	c = flag.Bool("c", false, "clear")
	r = flag.Bool("r", false, "restart")
)

func cls() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

type gotr struct {
	cmd   *exec.Cmd
	going bool
}

func (g *gotr) run(n string) {
	m.Lock()
	if g.going {
		return
	}
	g.going = true
	m.Unlock()

	if *r && g.cmd != nil {
		if g.cmd.ProcessState == nil || !g.cmd.ProcessState.Exited() {
			if g.cmd.Process != nil {
				if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
					g.cmd.Process.Kill()
				}
				g.cmd.Process.Wait()
			}
		}
	}
	g.cmd = exec.Command(flag.Arg(0))
	args := make([]string, flag.NArg())
	for i := 0; i < len(args); i++ {
		args[i] = flag.Arg(i)
		if args[i] == "/_" {
			args[i] = n
		}
	}
	g.cmd.Args = args
	g.cmd.Stdout = os.Stdout
	g.cmd.Stderr = os.Stderr
	if *c {
		cls()
	}
	if err := g.cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, os.Args[0], err)
	}

	m.Lock()
	g.going = false
	m.Unlock()
}

func main() {
	flag.Parse()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintln(os.Stderr, os.Args[0], err)
		os.Exit(1)
	}

	files := map[string]bool{}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			n := scanner.Text()
			m.Lock()
			if _, ok := files[n]; !ok {
				if fn, err := filepath.Abs(n); err == nil {
					// handle long name
					files[fn] = true
					w.Add(n)
				}
			}
			m.Unlock()
		}
	}()

	app := new(gotr)
	for {
		select {
		case ev := <-w.Events:
			if ev.Op == fsnotify.Create || ev.Op == fsnotify.Rename {
				w.Add(ev.Name)
			}
			fn, err := filepath.Abs(ev.Name)
			if err != nil {
				continue
			}
			m.Lock()
			// check as long name
			if _, ok := files[fn]; !ok {
				m.Unlock()
				continue
			}
			m.Unlock()
			go app.run(ev.Name)
		case err := <-w.Errors:
			fmt.Fprintln(os.Stderr, os.Args[0], err)
		}
	}
}
