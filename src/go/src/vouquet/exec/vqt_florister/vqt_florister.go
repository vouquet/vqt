package main

import (
	"os"
	"fmt"
	"flag"
	"time"
	"context"
	"strconv"
)

import (
	"vouquet/lock"
	"vouquet/farm"
	"vouquet/vouquet"
)

const (
	SELF_NAME string = "vqt_florister"
	USAGE string = "[-version] [-c <config path>] <NAMEofFlorist> <SEED> <SOIL> <SIZE>"
)

var (
	Version string

	Cpath  string

	Name   string
	Seed string
	Soil   string
	Size   float64
)

type logger struct {}

func (self *logger) WriteMsg(s string, msg ...interface{}) {
	tstr := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(os.Stdout, tstr + " " + s + "\n" , msg...)
}

func (self *logger) WriteErr(s string, msg ...interface{}) {
	tstr := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(os.Stderr, tstr + " " + s + "\n" , msg...)
}

func (self *logger) WriteDebug(s string, msg ...interface{}) {
	tstr := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(os.Stdout, tstr + " [DEBUG] " + s + "\n" , msg...)
}

func florister() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := new(logger)

	r, err := farm.OpenRegistry(Cpath, ctx, log)
	if err != nil {
		return err
	}
	defer r.Close()

	now := time.Now()
	start := now.AddDate(0, 0, -1)
	init_status, err := r.GetStatus(Soil, Seed, start, now)
	if err != nil {
		return err
	}

	pl, err := farm.NewFlowerpot(Soil, Seed, Cpath, ctx, log)
	if err != nil {
		return err
	}
	fl, err := vouquet.NewFlorist(Name, pl, init_status, log)
	if err != nil {
		return err
	}
	defer fl.Release()
	fl.SetSize(Size)

	st_ch := make(chan *farm.State)
	go func() {
		defer close(st_ch)

		mtx := lock.NewTryMutex(ctx)
		t := time.NewTicker(time.Second)
		var before time.Time
		for {
			select {
			case <- ctx.Done():
				return
			case <-t.C:
				go func() {
					ok, err := mtx.TryLock()
					if err != nil {
						if err == lock.ERR_CONTEXT_CANCEL {
							return
						}
						log.WriteErr("Cannot lock: '%s'", err)
						return
					}
					if !ok {
						return
					}
					defer mtx.Unlock()

					state, err := r.GetLastState(Soil, Seed)
					if err != nil {
						log.WriteErr("Cannot get status: '%s'", err)
						return
					}
					if state.Date().Equal(before) {
						log.WriteErr("Same the time in state.")
						return
					}
					before = state.Date()

					select {
					case <- ctx.Done():
						return
					case st_ch <- state:
					}
				}()
			}
		}
	}()

	log.WriteMsg("Start %s %s", SELF_NAME, Version)
	if err := fl.Run(ctx, st_ch); err != nil {
		return err
	}
	return nil
}

func die(s string, msg ...interface{}) {
	fmt.Fprintf(os.Stderr, s + "\n" , msg...)
	os.Exit(1)
}

func init() {
	var c_path string
	var see_version bool
	flag.StringVar(&c_path, "c", "./vouquet.conf", "config path.")
	flag.BoolVar(&see_version, "version", false, "display version.")
	flag.Parse()

	if see_version {
		fmt.Printf("Version: %s %s\n", SELF_NAME, Version)
		os.Exit(0)
	}

	if flag.NArg() < 4 {
		die("usage : %s %s", SELF_NAME, USAGE)
	}

	name := flag.Arg(0)
	seed := flag.Arg(1)
	soil := flag.Arg(2)
	size, err := strconv.ParseFloat(flag.Arg(3), 64)
	if err != nil {
		die("cannot convert size: '%s", err)
	}

	if c_path == "" {
		die("empty path")
	}

	Cpath = c_path
	Name = name
	Seed = seed
	Soil = soil
	Size = size
}

func main() {
	if err := florister(); err != nil {
		die("%s", err)
	}

}

