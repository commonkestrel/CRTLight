package main

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "image/color"
    "math"
    "os"
    "strings"
    "time"
    "image"
    
    _ "image/png"

    "github.com/faiface/pixel"
    "github.com/faiface/pixel/imdraw"
    "github.com/faiface/pixel/pixelgl"
    "github.com/faiface/pixel/text"
    "github.com/go-vgo/robotgo/clipboard"
    "github.com/sqweek/dialog"
    "github.com/nfnt/resize"
    "golang.org/x/image/colornames"
    "golang.org/x/image/font/basicfont"
)

const (
    DIAGONAL = 29.21
    xStart   = (1920 - 1200) / 2.0
    yStart   = (1080 - 900) / 2.0
    yCm      = 3 / 5.0 * DIAGONAL
    xCm      = 4 / 5.0 * DIAGONAL
    stepCm   = 100/60.0

    CIRSIZE = 40
)

var (
    win *pixelgl.Window
    imd *imdraw.IMDraw

    Text     string
    coloring = -1
    ZL       = light{}
    fps      = 4
    anim     bool

    frames = [][]*light{}
    frame  *[]*light
    fn     int
    ZF     []*light
)

type arrow struct {
    Min, Max  pixel.Vec
    Pos, Size pixel.Vec
    Color     color.RGBA
}

func NewArrow(pos, size pixel.Vec) *arrow {
    var min, max pixel.Vec
    if size.X < 0 {
        min.X = pos.X + size.X
        max.X = pos.X
    } else {
        min.X = pos.X
        max.X = pos.X + size.X
    }

    if size.Y < 0 {
        min.Y = pos.Y + size.Y
        max.Y = pos.Y
    } else {
        min.Y = pos.Y
        max.Y = pos.Y + size.Y
    }
    return &arrow{min, max, pos, size, colornames.White}
}

func (a *arrow) Contains(point pixel.Vec) bool {
    return a.Min.X <= point.X && point.X <= a.Max.X && a.Min.Y <= point.Y && point.Y <= a.Max.Y
}

func (a *arrow) Draw() {
    imd.Color = a.Color
    imd.Push(a.Pos)
    imd.Push(pixel.V(a.Pos.X+a.Size.X, a.Pos.Y+a.Size.Y/2))
    imd.Line(5)

    imd.Push(pixel.V(a.Pos.X+a.Size.X, a.Pos.Y+a.Size.Y/2))
    imd.Push(pixel.V(a.Pos.X, a.Pos.Y+a.Size.Y))
    imd.Line(5)

    imd.Push(pixel.V(a.Pos.X+a.Size.X, a.Pos.Y+a.Size.Y/2))
    imd.Circle(2.5, 0)
}

type light struct {
    Color color.RGBA
    Pos   pixel.Vec
    Index [2]int
}

func NewLight(pos pixel.Vec, rgb color.RGBA, x, y int) *light {
    return &light{rgb, pos, [2]int{x, y}}
}

func (l *light) Draw() {
    glow(l.Pos, CIRSIZE, pixel.ToRGBA(l.Color))
}

func frameInit(frame *[]*light) {
    *frame = []*light{}
    for y := 0.0; y < yCm/stepCm; y++ {
        for x := 0.0; x < xCm/stepCm; x++ {
            rgb := colornames.Black
            pos := pixel.V((x/(xCm/stepCm)*1200)+xStart+1, (y/(yCm/stepCm)*900)+yStart+22)

            l := NewLight(pos, rgb, int(x), int(y))
            *frame = append(*frame, l)
        }
    }
}

func Save() {
    path, err := dialog.File().Filter("Json File (*.json)", "json").Save()
    if err != nil {
        return
    }

    if !strings.Contains(path, ".json") {
        path += ".json"
    }

    file, _ := os.Create(path)
    marshaled, err := json.Marshal(frames)
    if err != nil {
        return
    }

    file.Write(marshaled)
}

func Load() {
    var opened [][]*light
    path, err := dialog.File().Filter("Json File (*.json)", "json").Load()
    if err != nil {
        return
    }

    file, _ := os.ReadFile(path)

    json.Unmarshal(file, &opened)
    frames = opened
    fn = 0
    frame = &frames[fn]
}

func Image() {
    path, err := dialog.File().Filter("PNG Files (*.png)", "png").Load()
    if err != nil {
        return
    }

    file, err := os.Open(path)
    if err != nil {
        dialog.Message("Failed to open file.").Error()
    }

    img, _, err := image.Decode(file)
    if err != nil {
        dialog.Message("Failed to decode image. Make sure the image is in an acceptable format.").Error()
    }

    resized := resize.Resize(uint(math.Floor(xCm/stepCm))+1, uint(math.Floor(yCm/stepCm))+1, img, resize.NearestNeighbor)

    for y := int(math.Floor(yCm/stepCm)+1)-1; y >= 0; y-- {
        for x := 0; x < int(math.Floor(xCm/stepCm)+1); x++ {
                pix := resized.At(x, int(math.Floor(yCm/stepCm))-y)
                r, g, b, _ := pix.RGBA()
                r /= 256
                g /= 256
                b /= 256

                i := y * int(math.Floor(xCm/stepCm)+1) + x
                (*frame)[i].Color = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
        }
    }
}

func Anim(stop chan bool) {
    win.UpdateInput()

    ticker := time.NewTicker(time.Second / time.Duration(fps))
    defer ticker.Stop()
    prevFps := fps

    for {
        select {
        case <-stop:
            return
        default:
            if prevFps != fps {
                ticker.Reset(time.Second / time.Duration(fps))
                prevFps = fps
            }

            fn++
            if fn > len(frames)-1 {
                fn = 0
                frame = &frames[0]
            } else {
                frame = &frames[fn]
            }

            <-ticker.C
        }
    }
}

func loadPicture(path string) (pixel.Picture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return pixel.PictureDataFromImage(img), nil
}

func run() {
    frameInit(&ZF)

    monitor := pixelgl.PrimaryMonitor()
    PositionX, PositionY := monitor.Position()
    SizeX, SizeY := monitor.Size()
    screen := pixel.R(PositionX, PositionY, SizeX, SizeY)

    icon, _ := loadPicture("C:\\Users\\Jett\\Documents\\Scripts\\CRTLight\\icon.png")

    cfg := pixelgl.WindowConfig{
        Title:     "CRT",
        Bounds:    screen,
        Resizable: false,
        Maximized: true,
        Icon: []pixel.Picture{icon},
    }
    var err error
    win, err = pixelgl.NewWindow(cfg)
    if err != nil {
        panic(err)
    }

    imd = imdraw.New(nil)
    ticker := time.NewTicker(time.Second / 60)
    defer ticker.Stop()

    atlas := text.NewAtlas(
        basicfont.Face7x13,
        []rune("0123456789ABCDEF"),
    )
    txt := text.New(pixel.V(win.Bounds().Center().X, yStart/2), atlas)

    frameAtlas := text.NewAtlas(
        basicfont.Face7x13,
        []rune("0123456789fps "),
    )
    frameTxt := text.New(pixel.V(screen.Center().X, yStart+910), frameAtlas)
    speedUp := NewArrow(pixel.V(screen.Center().X, yStart+900), pixel.V(20, 40))
    slowDown := NewArrow(pixel.V(screen.Center().X, yStart+900), pixel.V(-20, 40))

    frames = append(frames, []*light{})
    frame = &frames[fn]
    frameInit(frame)

    next := NewArrow(pixel.V(1920-50, 10), pixel.V(40, 80))
    back := NewArrow(pixel.V(50, 10), pixel.V(-40, 80))

    stop := make(chan bool)

    win.SetMousePosition(screen.Center())

    for !win.Closed() {
        mouse := win.MousePosition()
        leftPress := win.JustPressed(pixelgl.MouseButtonLeft)

        if leftPress && next.Contains(mouse) {
            fn++
            if len(frames)-1 < fn {
                newFrame := []*light{}
                frameInit(&newFrame)
                frames = append(frames, newFrame)
            }
            frame = &frames[fn]
        }

        if leftPress && back.Contains(mouse) {
            if fn > 0 {
                empty := true
                for _, l := range *frame {
                    if l.Color != colornames.Black {
                        empty = false
                        break
                    }
                }
                if empty && fn == len(frames)-1 {
                    frames = frames[:len(frames)-1]
                }

                fn--
                frame = &frames[fn]
            } else {
                fn = len(frames)-1
                frame = &frames[fn]
            }
        }

        win.Clear(color.RGBA{30, 30, 30, 0xFF})
        imd.Clear()
        txt.Clear()
        frameTxt.Clear()

        if win.JustPressed(pixelgl.KeyEscape) {
            if coloring == -1 {
                win.SetClosed(true)
            }
            coloring = -1
            fmt.Fprint(txt, Text)
            Text = ""
        }

        imd.Color = colornames.Black
        imd.Push(pixel.V(xStart, yStart))
        imd.Push(pixel.V(xStart+1200, yStart+900))
        imd.Rectangle(0)

        for i, l := range *frame {
            if l.Color != colornames.Black {
                l.Draw()
            }
            if l.Pos.Sub(mouse).Len() <= CIRSIZE && mouse.X > xStart && mouse.Y > yStart && mouse.X < xStart+1200 && mouse.Y < yStart+900 {
                if win.JustPressed(pixelgl.MouseButtonLeft) && coloring != i {
                    coloring = i
                    fmt.Fprint(txt, Text)
                    Text = ""
                }
                if win.JustPressed(pixelgl.MouseButtonRight) {
                    l.Color = colornames.Black
                    if i == coloring {
                        coloring = -1
                        Text = ""
                    }
                }

                if (win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl)) && win.JustPressed(pixelgl.KeyC) {
                    clipboard.WriteAll(fmt.Sprintf("#%.2X%.2X%.2X", l.Color.R, l.Color.G, l.Color.B))
                }

                if coloring == -1 && l.Color != colornames.Black {
                    hex := fmt.Sprintf("%.2X%.2X%.2X", l.Color.R, l.Color.G, l.Color.B)
                    fmt.Fprint(txt, hex)
                }
                imd.Color = colornames.White
                imd.Push(l.Pos)
                imd.Circle(CIRSIZE, 1)
            }

        }

        if (win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl)) && win.JustPressed(pixelgl.KeyV) {
            if coloring != -1 {
                clip, _ := clipboard.ReadAll()
                if clip[0] == '#' {
                    clip = clip[1:]
                }
                clip = strings.ToUpper(clip)

                if len(Text)+len(clip) <= 6 {
                    Text += clip
                }
            } else if fn > 0 {
                previous := frames[fn-1]
                for i, f := range *frame {
                    *f = *previous[i]
                }
            }
        }

        if coloring != -1 {
            if len(Text) < 6 {
                Text += HexIn()
            }
            if win.JustPressed(pixelgl.KeyBackspace) && len(Text) > 0 {
                Text = Text[:len(Text)-1]
            }
            if win.JustPressed(pixelgl.KeyEnter) && len(Text) == 6 {
                decode, _ := hex.DecodeString(Text)
                rgb := color.RGBA{decode[0], decode[1], decode[2], 0xFF}
                (*frame)[coloring].Color = rgb
                coloring = -1
                fmt.Fprint(txt, Text)
                Text = ""
            }

            if (win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl)) && win.JustPressed(pixelgl.KeyV) {
                clip, _ := clipboard.ReadAll()
                if clip[0] == '#' {
                    clip = clip[1:]
                }
                clip = strings.ToUpper(clip)

                if len(Text)+len(clip) <= 6 {
                    Text += clip
                }
            }
        }

        if win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl) {
            if win.JustPressed(pixelgl.KeyS) {
                go Save()
            }
            if win.JustPressed(pixelgl.KeyF) {
                go Load()
            }
            if win.JustPressed(pixelgl.KeyD) {
                if fn == len(frames)-1 {
                    frames = frames[:len(frames)-1]
                    fn--
                    frame = &frames[fn]
                } else if fn == 0 {
                    frameInit(frame)
                } else {
                    frames = append(frames[:fn], frames[fn+1:]...)
                    frame = &frames[fn]
                }
            }
            if win.JustPressed(pixelgl.KeyI) {
                Image()
            }
        }

        if win.JustPressed(pixelgl.KeySpace) {
            if !anim {
                go Anim(stop)
            } else {
                stop <- true
            }
            anim = !anim
        }

        imd.Color = color.RGBA{30, 30, 30, 0xFF}
        imd.Push(pixel.V(0, 0))
        imd.Push(pixel.V(1920, yStart))
        imd.Rectangle(0)

        imd.Push(pixel.V(0, 0))
        imd.Push(pixel.V(xStart, 1080))
        imd.Rectangle(0)

        imd.Push(pixel.V(0, yStart+900))
        imd.Push(pixel.V(1920, 1080))
        imd.Rectangle(0)

        imd.Push(pixel.V(xStart+1200, 0))
        imd.Push(pixel.V(1920, 1080))
        imd.Rectangle(0)

        fmt.Fprint(txt, Text)

        fmt.Fprintf(frameTxt, "%v fps", fps)
        frameTxt.Orig.X = screen.Center().X - frameTxt.Bounds().Size().X/2

        speedUp = NewArrow(pixel.V(frameTxt.Bounds().Center().X+frameTxt.Bounds().Size().X*2.5+20, frameTxt.Orig.Y), speedUp.Size)
        slowDown = NewArrow(pixel.V(frameTxt.Bounds().Center().X-frameTxt.Bounds().Size().X*2.5-20, frameTxt.Orig.Y), slowDown.Size)

        if leftPress {
            if speedUp.Contains(mouse) {
                fps++
            } else if leftPress && slowDown.Contains(mouse) {
                fps--
            }

            frameTxt.Clear()
            fmt.Fprintf(frameTxt, "%v fps", fps)
            frameTxt.Orig.X = screen.Center().X - frameTxt.Bounds().Size().X/2

            speedUp = NewArrow(pixel.V(frameTxt.Bounds().Center().X+frameTxt.Bounds().Size().X*2.5+20, frameTxt.Orig.Y), speedUp.Size)
            slowDown = NewArrow(pixel.V(frameTxt.Bounds().Center().X-frameTxt.Bounds().Size().X*2.5-20, frameTxt.Orig.Y), slowDown.Size)
        }

        next.Draw()
        back.Draw()
        speedUp.Draw()
        slowDown.Draw()

        imd.Draw(win)
        txt.Draw(win, pixel.IM.Scaled(txt.Bounds().Center(), 8))
        frameTxt.Draw(win, pixel.IM.Scaled(pixel.V(frameTxt.Bounds().Center().X, frameTxt.Orig.Y), 5))
        win.Update()
        <-ticker.C
    }
}

func HexIn() string {
    if win.JustPressed(pixelgl.Key0) {
        return "0"
    } else if win.JustPressed(pixelgl.Key1) {
        return "1"
    } else if win.JustPressed(pixelgl.Key2) {
        return "2"
    } else if win.JustPressed(pixelgl.Key3) {
        return "3"
    } else if win.JustPressed(pixelgl.Key4) {
        return "4"
    } else if win.JustPressed(pixelgl.Key5) {
        return "5"
    } else if win.JustPressed(pixelgl.Key6) {
        return "6"
    } else if win.JustPressed(pixelgl.Key7) {
        return "7"
    } else if win.JustPressed(pixelgl.Key8) {
        return "8"
    } else if win.JustPressed(pixelgl.Key9) {
        return "9"
    } else if win.JustPressed(pixelgl.KeyA) {
        return "A"
    } else if win.JustPressed(pixelgl.KeyB) {
        return "B"
    } else if win.JustPressed(pixelgl.KeyC) {
        return "C"
    } else if win.JustPressed(pixelgl.KeyD) {
        return "D"
    } else if win.JustPressed(pixelgl.KeyE) {
        return "E"
    } else if win.JustPressed(pixelgl.KeyF) {
        return "F"
    }
    return ""
}

func glow(pos pixel.Vec, radius float64, rgb pixel.RGBA) {
    rroot := math.Pow(2, 1/radius)
    for i := radius; i >= 0; i-- {
        percent := math.Pow(rroot, radius-i) - 1
        if percent > 1 {
            percent = 1
        }

        resultRed, resultGreen, resultBlue := percent*rgb.R, percent*rgb.G, percent*rgb.B
        imd.Color = pixel.RGB(resultRed, resultGreen, resultBlue)
        imd.Push(pos)
        imd.Circle(i, 0)
    }
}

func main() {
    pixelgl.Run(run)
}
