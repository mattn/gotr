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
					files[fn] = true
					w.Add(n)
				}
			}
			m.Unlock()
		}
	}()

	var cmd *exec.Cmd
	var going bool

	for {
		select {
		case ev := <-w.Events:
			if ev.Op&fsnotify.Rename == fsnotify.Rename {
				w.Add(ev.Name)
			}
			fn, err := filepath.Abs(ev.Name)
			if err != nil {
				continue
			}
			m.Lock()
			if _, ok := files[fn]; !ok {
				m.Unlock()
				continue
			}
			m.Unlock()
			if ev.Op == fsnotify.Create || ev.Op == fsnotify.Rename {
				w.Add(ev.Name)
			}
			go func(n string) {
				m.Lock()
				defer func() {
					going = false
					m.Unlock()
				}()
				if going {
					return
				}
				going = true

				if *r && cmd != nil {
					if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
						if cmd.Process != nil {
							if err := cmd.Process.Signal(os.Interrupt); err != nil {
								cmd.Process.Kill()
							}
							cmd.Process.Wait()
						}
					}
				}
				cmd = exec.Command(flag.Arg(0))
				args := make([]string, flag.NArg())
				for i := 0; i < len(args); i++ {
					args[i] = flag.Arg(i)
					if args[i] == "/_" {
						args[i] = n
					}
				}
				cmd.Args = args
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if *c {
					cls()
				}
				if err := cmd.Start(); err != nil {
					fmt.Fprintln(os.Stderr, os.Args[0], err)
				}
			}(ev.Name)
		case err := <-w.Errors:
			fmt.Fprintln(os.Stderr, os.Args[0], err)
		}
	}
}
