package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/PerformLine/go-stockutil/colorutil"
	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/gen2brain/cam2ip/camera"
	"github.com/joho/godotenv"
	"github.com/nfnt/resize"
)

const (
	RED   = 17
	GREEN = 24
	BLUE  = 22
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
			cam, err = camera.New(camera.Options{Width: 640, Height: 480})
			if err != nil {
				log.Panic(err)
			}

			go func() {
				for {
					if cam == nil {
						log.Println("camera is nil, stopping")
						break
					}

					img, err := cam.Read()
					if err != nil {
						log.Printf("failed to get next frame: %v\n", err)
						continue
					}

					img = resize.Resize(80, 80, img, resize.NearestNeighbor)

					totalR := uint32(0)
					totalG := uint32(0)
					totalB := uint32(0)
					for y := 0; y < 80; y++ {
						for x := 0; x < 80; x++ {
							r, g, b, _ := img.At(x, y).RGBA()
							r /= 257
							g /= 257
							b /= 257

							totalR += r
							totalG += g
							totalB += b
						}
					}

					r := totalR / (80 * 80)
					g := totalG / (80 * 80)
					b := totalB / (80 * 80)

					h, s, l := colorutil.RgbToHsl(float64(r), float64(g), float64(b))
					s = math.Min(s*2, 1) // Increase saturation

					rf, gf, bf := colorutil.HslToRgb(h, s, l)
					r = uint32(rf)
					g = uint32(gf)
					b = uint32(bf)

					log.Printf("setting color: rgb(%d, %d, %d)\n", r, g, b)

					SetPin(RED, r)
					SetPin(GREEN, g)
					SetPin(BLUE, b)
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
