package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/mattn/go-tty"
	"github.com/urfave/cli/v2"
)

var quitChan = make(chan struct{})

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"H"},
				Value:   "0.0.0.0",
				Usage:   "listening host",
			},
			&cli.Int64Flag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8080,
				Usage:   "listening port",
			},
		},
		Name:   "gprogen",
		Usage:  "Go proxy and stub generator for load test",
		Action: Start,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("%+v\n", err)
	}
}

func Start(c *cli.Context) error {
	svc := http.Server{
		Addr: c.String("host") + ":" + c.String("port"),
	}

	t, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer t.Close()
	go func(t *tty.TTY) {
		for {
			r, err := t.ReadRune()
			if err != nil {
				log.Println(err)
			}
			if r == 'q' {
				if err := svc.Shutdown(context.Background()); err != nil {
					log.Println(err)
				}
				close(quitChan)
				break
			}
			log.Printf("Press q to quit (pressed %v).\n", string(r))
		}
	}(t)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, world!"))
	})

	log.Println("listening on", svc.Addr)
	if err := svc.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	<-quitChan
	return nil
}
