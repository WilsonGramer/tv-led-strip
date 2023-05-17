package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/gen2brain/cam2ip/camera"
	"github.com/joho/godotenv"
)

const (
	RED   = 17
	GREEN = 22
	BLUE  = 24
)

func main() {
	err := godotenv.Load()
	if err != nil {
		if err, ok := err.(*os.PathError); ok {
			log.Printf(".env not present: %v\n", err)
		} else {
			log.Fatal("error loading .env file")
		}
	}

	info := accessory.Info{
		Name:         os.Getenv("ACCESSORY_NAME"),
		Manufacturer: os.Getenv("ACCESSORY_MANUFACTURER"),
		SerialNumber: os.Getenv("ACCESSORY_SERIAL_NUMBER"),
		Model:        os.Getenv("ACCESSORY_MODEL"),
	}

	log.Printf("accessory info: %v\n", info)

	accessory := accessory.NewSwitch(info)

	var cam *camera.Camera
	accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on {
			log.Println("turning on")

			var err error
			cam, err = camera.New(camera.Options{Width: prominentcolor.DefaultSize, Height: prominentcolor.DefaultSize})
			if err != nil {
				log.Panic(err)
			}

			go func() {
				for {
					if cam == nil {
						break
					}

					img, err := cam.Read()
					if err != nil {
						log.Printf("failed to get next frame: %v\n", err)
						continue
					}

					centroids, err := prominentcolor.KmeansWithAll(1, img, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, []prominentcolor.ColorBackgroundMask{})
					if err != nil {
						log.Printf("failed to get dominant color: %v\n", err)
						continue
					}

					color := centroids[0]

					log.Printf("setting color: rgb(%d, %d, %d)\n", color.Color.R, color.Color.G, color.Color.B)

					SetPin(RED, color.Color.R)
					SetPin(GREEN, color.Color.G)
					SetPin(BLUE, color.Color.B)
				}
			}()
		} else {
			log.Println("turning off")

			if cam != nil {
				cam.Close()
				cam = nil
			}

			go func() {
				time.Sleep(500 * time.Millisecond)

				SetPin(RED, 0)
				SetPin(GREEN, 0)
				SetPin(BLUE, 0)
			}()
		}
	})

	storePath := fmt.Sprintf("%s/.config/tv-led-strip", os.Getenv("HOME"))
	log.Printf("fs store path: %s", storePath)

	fs := hap.NewFsStore(storePath)

	server, err := hap.NewServer(fs, accessory.A)
	if err != nil {
		log.Panic(err)
	}

	server.Pin = os.Getenv("PIN")

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c

		signal.Stop(c)
		cancel()
	}()

	err = server.ListenAndServe(ctx)
	if err != nil {
		log.Panic(err)
	}
}

func SetPin(pin uint, value uint32) {
	_, err := exec.Command("pigs", "p", fmt.Sprint(pin), fmt.Sprint(value)).Output()
	if err != nil {
		log.Printf("failed to set pin %v: %v: %v\n", pin, err, string(err.(*exec.ExitError).Stderr))
	}
}
