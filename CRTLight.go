package main

import (
    "encoding/hex"
    "encoding/json"
    "fmt"
    "image"
    "image/color"
    "math"
    "os"
    "strings"
    "time"

    _ "image/png"

    "github.com/faiface/pixel"
    "github.com/faiface/pixel/imdraw"
    "github.com/faiface/pixel/pixelgl"
    "github.com/faiface/pixel/text"
    "github.com/go-vgo/robotgo/clipboard"
    "github.com/nfnt/resize"
    "github.com/sqweek/dialog"
    "golang.org/x/image/colornames"
    "golang.org/x/image/font/basicfont"
)

const (
    DIAGONAL = 29.21
    xStart   = (1920 - 1200) / 2.0
    yStart   = (1080 - 900) / 2.0
    yCm      = 3 / 5.0 * DIAGONAL
    xCm      = 4 / 5.0 * DIAGONAL
    stepCm   = 100 / 60.0

    CIRSIZE = 40
)

type export struct {
    Frames    []Frame
    Fps       int
    Loop      bool
    LoopDelay time.Duration
}

var (
    win *pixelgl.Window
    imd *imdraw.IMDraw

    Text string

    ncolor   = pixel.V(-1, -1)
    coloring = ncolor

    ZL   color.Color
    fps  = 4
    anim bool

    frames []Frame
    frame  *Frame
    fn     int
    ZF     Frame
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

func Draw(pos pixel.Vec, rgb color.Color) {
    glow(pos, CIRSIZE, pixel.ToRGBA(rgb))
}

type Frame [15][11]color.RGBA

func (f *Frame) Draw() {
    for x := 0; x < len(f); x++ {
        for y := len(f[x]) - 1; y >= 0; y-- {
            l := f[x][y]
            if l != colornames.Black {
                pos := pixel.V((float64(x)/14*1200)+xStart+1, (float64(y)/10.0*900.0)+yStart+22)
                Draw(pos, l)
            }
        }
    }
}

func (f *Frame) Init() {
    for x, col := range f {
        for y := range col {
            f[x][y] = colornames.Black
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

    ex := export{
        Frames: frames,
        Fps:    fps,
    }

    file, _ := os.Create(path)
    marshaled, err := json.Marshal(ex)
    if err != nil {
        return
    }

    file.Write(marshaled)
}

func Load() {
    var opened export
    path, err := dialog.File().Filter("Json File (*.json)", "json").Load()
    if err != nil {
        return
    }

    file, _ := os.ReadFile(path)

    json.Unmarshal(file, &opened)
    fps = opened.Fps
    frames = opened.Frames
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

    for y := int(math.Floor(yCm/stepCm)+1) - 1; y >= 0; y-- {
        for x := 0; x < int(math.Floor(xCm/stepCm)+1); x++ {
            pix := resized.At(x, int(math.Floor(yCm/stepCm))-y)
            r, g, b, _ := pix.RGBA()
            r /= 256
            g /= 256
            b /= 256

            frame[x][y] = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
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
    ZF.Init()

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
        Icon:      []pixel.Picture{icon},
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

    frames = append(frames, Frame{})
    frame = &frames[fn]
    frame.Init()
    fmt.Println(len(frame))

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
                newFrame := Frame{}
                newFrame.Init()
                frames = append(frames, newFrame)
            }
            frame = &frames[fn]
        }

        if leftPress && back.Contains(mouse) {
            if fn > 0 {
                empty := true
                for _, col := range *frame {
                    for _, l := range col {
                        if l != colornames.Black {
                            empty = false
                            break
                        }
                    }
                }
                if empty && fn == len(frames)-1 {
                    frames = frames[:len(frames)-1]
                }

                fn--
                frame = &frames[fn]
            } else {
                fn = len(frames) - 1
                frame = &frames[fn]
            }
        }

        win.Clear(color.RGBA{30, 30, 30, 0xFF})
        imd.Clear()
        txt.Clear()
        frameTxt.Clear()

        if win.JustPressed(pixelgl.KeyEscape) {
            if coloring.Eq(ncolor) {
                win.SetClosed(true)
            } else {

                coloring = ncolor
                fmt.Fprint(txt, Text)
                Text = ""
            }
        }

        imd.Color = colornames.Black
        imd.Push(pixel.V(xStart, yStart))
        imd.Push(pixel.V(xStart+1200, yStart+900))
        imd.Rectangle(0)

        frame.Draw()

        for x, col := range *frame {
            for y, l := range col {
                pos := pixel.V((float64(x)/14*1200)+xStart+1, (float64(y)/10*900)+yStart+22)
                i := pixel.V(float64(x), float64(y))

                if pos.Sub(mouse).Len() <= CIRSIZE && mouse.X > xStart && mouse.Y > yStart && mouse.X < xStart+1200 && mouse.Y < yStart+900 {
                    if win.JustPressed(pixelgl.MouseButtonLeft) && coloring != i {
                        coloring = i
                        fmt.Fprint(txt, Text)
                        Text = ""
                    }
                    if win.JustPressed(pixelgl.MouseButtonRight) {
                        frame[x][y] = colornames.Black
                        if coloring.Eq(i) {
                            coloring = ncolor
                            Text = ""
                        }
                    }

                    if (win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl)) && win.JustPressed(pixelgl.KeyC) {
                        clipboard.WriteAll(fmt.Sprintf("#%.2X%.2X%.2X", l.R, l.G, l.B))
                    }

                    if coloring.Eq(ncolor) && l != colornames.Black {
                        hex := fmt.Sprintf("%.2X%.2X%.2X", l.R, l.G, l.B)
                        fmt.Fprint(txt, hex)
                    }
                    imd.Color = colornames.White
                    imd.Push(pos)
                    imd.Circle(CIRSIZE, 1)
                }
            }
        }

        if (win.Pressed(pixelgl.KeyLeftControl) || win.Pressed(pixelgl.KeyRightControl)) && win.JustPressed(pixelgl.KeyV) {
            if !coloring.Eq(ncolor) {
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
                for x := 0; x < len(frame); x++ {
                    for y := 0; y < len(frame[x]); y++ {
                        frame[x][y] = previous[x][y]
                    }
                }
            }
        }

        if !coloring.Eq(ncolor) {
            if len(Text) < 6 {
                Text += HexIn()
            }
            if win.JustPressed(pixelgl.KeyBackspace) && len(Text) > 0 {
                Text = Text[:len(Text)-1]
            }
            if win.JustPressed(pixelgl.KeyEnter) && len(Text) == 6 {
                decode, _ := hex.DecodeString(Text)
                rgb := color.RGBA{decode[0], decode[1], decode[2], 0xFF}
                (*frame)[int(coloring.X)][int(coloring.Y)] = rgb
                coloring = pixel.V(-1, -1)
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
                if fn == 0 {
                    frame.Init()
                } else if fn == len(frames)-1 {
                    frames = frames[:len(frames)-1]
                    fn--
                    frame = &frames[fn]
                } else {
                    frames = append(frames[:fn], frames[fn+1:]...)
                    frame = &frames[fn]
                }
            }
            if win.JustPressed(pixelgl.KeyA) {
                if fn != len(frames)-1 {
                    newframe := Frame{}
                    newframe.Init()
                    frames = append(frames[:fn+2], frames[fn+1:]...)
                    frames[fn+1] = newframe
                    fn++
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

        resultRed, resultGreen, resultBlue := percent*float64(rgb.R), percent*float64(rgb.G), percent*float64(rgb.B)
        imd.Color = pixel.RGB(resultRed, resultGreen, resultBlue)
        imd.Push(pos)
        imd.Circle(i, 0)
    }
}

func main() {
    pixelgl.Run(run)
}
