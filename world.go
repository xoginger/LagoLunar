// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"
)

// El continente: 3000 x 1200 celdas, generado una vez con semilla fija y guardado
// de forma compacta (tipo, bioma, glifo, RGB de 8 bits), unos 25 MB.
const W, H = 3000, 1200

const (
	tGrass uint8 = iota
	tForest
	tRock
	tSnow
	tLake
	tRiver
	tSea
	tSand
	tRoad
	tBridge
	tWall
	tFloor
	tLava
	tIce
	tPave // empedrado de pueblos y ciudades
)

const (
	bPlains uint8 = iota
	bForest
	bDark
	bRed
	bSnow
	bVolcanic
	bDesert
	bSwamp
	bCanyon
	bHaunted
	bThunder
	bCrater
)

var biomeNames = []string{"Llanuras del Centro", "Bosque Verde", "Bosque Umbrío", "Bosque Ascua", "Tundra Blanca",
	"Tierras de Ceniza", "Desierto de Ámbar", "Pantanos Místicos", "Cañones Rojos", "Páramo Embrujado", "Mesetas del Trueno", "Valle del Meteoro"}

var (
	ttype    []uint8
	biome    []uint8
	gch      []byte
	gcol     []uint8
	waterTop []int16
	coastX   [H]int
)

func idx(x, y int) int  { return y*W + x }
func inb(x, y int) bool { return x >= 0 && y >= 0 && x < W && y < H }

func cellAt(x, y int) Cell {
	i := idx(x, y)
	return Cell{rune(gch[i]), RGB{float64(gcol[3*i]), float64(gcol[3*i+1]), float64(gcol[3*i+2])}}
}
func setCell(x, y int, ch rune, c RGB) {
	if !inb(x, y) {
		return
	}
	i := idx(x, y)
	if ch > 126 {
		ch = '#'
	}
	gch[i] = byte(ch)
	gcol[3*i], gcol[3*i+1], gcol[3*i+2] = uint8(cl(c.R)), uint8(cl(c.G)), uint8(cl(c.B))
}

func isWaterT(t uint8) bool { return t == tLake || t == tRiver || t == tSea }
func isWater(x, y int) bool { return inb(x, y) && isWaterT(ttype[idx(x, y)]) }
func blocked(x, y int) bool {
	if x < 8 || x >= W-8 || y < 16 || y >= H-1 {
		return true
	}
	t := ttype[idx(x, y)]
	return t == tRock || t == tWall || t == tLava
}
func setT(x, y int, t uint8) {
	if inb(x, y) {
		ttype[idx(x, y)] = t
	}
}
func getT(x, y int) uint8 {
	if !inb(x, y) {
		return tRock
	}
	return ttype[idx(x, y)]
}

// ruido
func h2(x, y, s int) float64 {
	n := uint32(x*374761393 + y*668265263 + s*1442695041)
	n = (n ^ (n >> 13)) * 1274126177
	n ^= n >> 16
	return float64(n&0xffffff) / float64(0xffffff)
}
func vnoise(x, y float64, s int) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	fx, fy := x-xi, y-yi
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := h2(int(xi), int(yi), s), h2(int(xi)+1, int(yi), s)
	c, d := h2(int(xi), int(yi)+1, s), h2(int(xi)+1, int(yi)+1, s)
	return a + (b-a)*fx + (c-a)*fy + (a-b-c+d)*fx*fy
}
func fbm(x, y float64, s int) float64 {
	v, amp, tot := 0.0, 1.0, 0.0
	for o := 0; o < 4; o++ {
		v += vnoise(x, y, s+o*31) * amp
		tot += amp
		x, y, amp = x*2.03, y*2.03, amp*0.5
	}
	return v / tot
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// lugares
type Loc struct {
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Kind string `json:"kind"`
	R    int    `json:"r"`
}

const lakeCX, lakeCY, lakeRX, lakeRY = 1080, 640, 110, 42
const capX0, capY0, capX1, capY1 = 1240, 520, 1700, 780

var locs = []Loc{
	{"Lago Lunar", lakeCX, lakeCY, "lake", 150},
	{"Ciudad de Lumbria", (capX0 + capX1) / 2, (capY0 + capY1) / 2, "capital", 260},
	{"Tumba Helada", 170, 190, "crypt", 45},
	{"Cueva de Hielo", 400, 300, "cave", 45},
	{"Guarida del Invierno", 330, 470, "cave", 45},
	{"Volcán Ceniza", 780, 160, "volcano", 60},
	{"Monasterio de Ascua", 1150, 110, "monastery", 90},
	{"Cueva de las Colinas", 1180, 460, "cave", 45},
	{"Templo Eterno", 660, 560, "temple", 50},
	{"Mansión Embrujada", 690, 800, "mansion", 55},
	{"Casa del Ermitaño", 430, 830, "hut", 35},
	{"Guarida Colmillo de Fuego", 330, 960, "cave", 45},
	{"Pirámide del Juicio", 430, 1080, "pyramid", 60},
	{"Cueva de Fuego", 200, 1140, "cave", 45},
	{"Castillo Encantado", 820, 1000, "castle", 60},
	{"Cueva Danzante", 900, 1130, "cave", 45},
	{"Cueva de la Cascada", 1040, 790, "cave", 45},
	{"Arena de los Héroes", 1820, 610, "arena", 60},
	{"Templo Oculto", 1640, 210, "temple", 60},
	{"Templo Maldito", 1960, 70, "temple", 50},
	{"Cueva de la Tierra", 1880, 380, "cave", 45},
	{"Guarida Tierra Firme", 2080, 320, "cave", 45},
	{"Minas Umbrías", 1920, 500, "mine", 55},
	{"Corazón del Pantano", 2450, 160, "swamp", 120},
	{"Cala Pirata", 2700, 400, "cove", 70},
	{"Torre de la Guardia", 2560, 560, "tower", 40},
	{"Caverna de la Orilla", 2840, 650, "cave", 45},
	{"Cráter del Meteoro", 2290, 700, "crater", 110},
	{"La Cripta", 2620, 780, "crypt", 45},
	{"Castillo del Cielo", 2260, 900, "skycastle", 60},
	{"Carnaval Errante", 1980, 750, "carnival", 60},
	{"Campamento Orco", 1960, 920, "orc", 60},
	{"Minas de la Sabana", 1250, 880, "mine", 55},
	{"Paso de los Mercaderes", 1520, 1010, "pass", 90},
	{"Prisión Antigua", 1450, 1140, "prison", 50},
	{"Cueva del Trueno", 2160, 1030, "cave", 45},
	{"Guarida Garra de Trueno", 1960, 1120, "cave", 45},
	{"Aldea del Alba", 2620, 1000, "village", 150},
	{"Playa Dorada", 2700, 1150, "beach", 80},
	{"Cueva del Sauce", 1500, 420, "cave", 40},
	{"Santuario de Piedra", 900, 330, "temple", 45},
}

type seed struct {
	x, y int
	b    uint8
}

var seeds = []seed{
	{300, 330, bSnow}, {150, 150, bSnow}, {480, 520, bSnow},
	{780, 180, bVolcanic}, {250, 1120, bVolcanic},
	{1150, 260, bRed}, {1000, 120, bRed}, {1300, 380, bRed},
	{680, 820, bHaunted},
	{450, 1060, bDesert}, {380, 900, bDesert},
	{850, 970, bCanyon}, {1150, 1060, bCanyon}, {2700, 300, bCanyon}, {2780, 760, bCanyon},
	{2450, 170, bSwamp}, {2250, 120, bSwamp},
	{2290, 700, bCrater},
	{2150, 1010, bThunder}, {2350, 1080, bThunder},
	{2620, 1000, bDesert},
	{1700, 280, bForest}, {2000, 400, bForest}, {1450, 1080, bForest}, {650, 640, bForest}, {2500, 520, bForest},
	{1850, 120, bDark}, {1950, 820, bDark}, {1100, 760, bDark}, {560, 420, bDark},
	{1450, 640, bPlains}, {1700, 880, bPlains}, {1250, 900, bPlains}, {1900, 640, bPlains}, {2100, 560, bPlains},
}

// arte ASCII
var pine = []string{"   *   ", "  /^\\  ", " /^^^\\ ", " /^^^\\ ", "/^^^^^\\", "  |#|  "}
var oak = []string{" .%%%. ", "%%&%%&%", "%&%%%%%", " '%%%' ", "  |#|  "}
var deadTree = []string{" \\ | / ", "  \\|/  ", "   Y   ", "   |   "}
var palm = []string{"\\\\|//", "-=@=-", " /|\\ ", "  (  ", "   ) "}
var cactus = []string{" _ ", "| |_", "| |/", "| | "}
var reeds = []string{"| ,|", "|,||"}
var crystal = []string{" /\\ ", "/**\\", "\\**/"}
var ship = []string{"   |\\   ", "   | \\  ", "   |__\\ ", "\\______/"}
var volcano = []string{"      ,~,      ", "     / ~ \\     ", "    /  ~  \\    ", "   /   ~   \\   ", "  /    ~    \\  ", " /     ~     \\ ", "/_____________\\"}
var caveMouth = []string{"  ____  ", " /    \\ ", "/ #### \\", "| #### |"}
var wheel = []string{"  .-o-.  ", " o  |  o ", "o---+---o", " o  |  o ", "  '-o-'  ", "   /|\\   "}
var moonArt = []string{"   .-'''-.   ", "  /  o    \\  ", " |   ()  . | ", " |  .   () | ", "  \\   o   /  ", "   '-...-'   "}
var cloud = []string{"  .--.   ", " (    )_ ", "(_______)"}

func stamp(x0, y0 int, art []string, ok func(t uint8) bool, setType int, col func(ch rune, i, j int) RGB) bool {
	for i, row := range art {
		for j, ch := range []rune(row) {
			if ch == ' ' {
				continue
			}
			x, y := x0+j, y0+i
			if !inb(x, y) || (ok != nil && !ok(ttype[idx(x, y)])) {
				return false
			}
		}
	}
	for i, row := range art {
		for j, ch := range []rune(row) {
			if ch == ' ' {
				continue
			}
			x, y := x0+j, y0+i
			setCell(x, y, ch, col(ch, i, j))
			if setType >= 0 {
				ttype[idx(x, y)] = uint8(setType)
			}
		}
	}
	return true
}

func stampC(cx, cy int, art []string, setType int, col func(ch rune, i, j int) RGB) {
	stamp(cx-len([]rune(art[0]))/2, cy-len(art)/2, art, nil, setType, col)
}

func solid(c RGB) func(rune, int, int) RGB { return func(rune, int, int) RGB { return c } }

// glifos del suelo
var grassGl = []byte{'|', '/', '\\', ',', '.', '\'', '|', ',', '"', '`'}

func ground(x, y int) {
	i := idx(x, y)
	n := h2(x, y, 3)
	fx, fy := float64(x), float64(y)
	b := biome[i]
	var ch byte = '.'
	var c RGB
	switch ttype[i] {
	case tLake, tRiver:
		ch, c = ' ', waterBlue
	case tSea:
		ch, c = ' ', RGB{4, 14, 40}
	case tRoad:
		v := 95 + n*40
		ch, c = []byte{'=', '#', '.', '=', ':'}[int(n*29)%5], RGB{v, v * 0.85, v * 0.6}
		if b == bSnow {
			c = RGB{v + 60, v + 60, v + 70}
		}
	case tBridge:
		ch, c = '#', RGB{150, 105, 60}
	case tPave:
		v := 125 + n*40
		ch, c = []byte{'.', ',', '_', '.', ':', ' ', '\''}[int(n*41)%7], RGB{v, v * 0.95, v * 0.88}
	case tLava:
		ch, c = []byte{'~', '~', '=', '-'}[int(n*7)%4], RGB{230 + n*25, 70 + n*80, 20}
	case tIce:
		ch, c = []byte{'_', '-', '.', ' '}[int(n*9)%4], RGB{170, 210, 240}
	case tSand:
		v := 150 + n*50
		ch, c = []byte{'.', ':', '`', '.', ' '}[int(n*37)%5], RGB{v, v * 0.88, v * 0.6}
	case tFloor:
		v := 70 + n*40
		ch, c = []byte{'.', ',', ' ', '_', '.'}[int(n*23)%5], RGB{v, v * 0.9, v * 0.75}
	case tRock:
		hgt := fbm(fx/9, fy/9, 81)
		v := 70 + hgt*90
		ch = []byte{'#', '%', '=', '#', '|', '^'}[int(n*71)%6]
		switch b {
		case bCanyon:
			c = RGB{v * 1.4, v * 0.6, v * 0.35}
		case bThunder:
			c = RGB{v * 0.6, v * 0.85, v * 1.2}
		case bCrater:
			c = RGB{v * 1.3, v * 0.6, v * 0.75}
		case bSnow:
			c = RGB{v + 70, v + 75, v + 90}
			ch = []byte{'^', '/', '\\', '^', 'A'}[int(n*71)%5]
		case bVolcanic:
			c = RGB{v * 0.5, v * 0.45, v * 0.45}
			ch = []byte{'^', '/', '\\', 'A', '#'}[int(n*71)%5]
		default:
			c = RGB{v, v * 0.95, v * 0.9}
			ch = []byte{'^', '/', '\\', 'A', '^', 'n'}[int(n*71)%6]
		}
	case tSnow:
		v := 200 + n*55
		ch, c = []byte{'.', '*', ' ', '\'', '.', ','}[int(n*61)%6], RGB{v * 0.92, v * 0.96, v}
	default: // pasto o suelo de bosque según bioma
		gl := grassGl[int(n*97)%len(grassGl)]
		ch = gl
		switch b {
		case bPlains:
			g := 75 + n*55
			c = RGB{g * 0.75, g, 30}
		case bForest:
			g := 50 + n*50
			c = RGB{15 + n*15, g, 22}
		case bDark:
			g := 35 + n*35
			c = RGB{10, g, g * 0.7}
		case bRed:
			v := 80 + n*60
			c = RGB{v, v * 0.35, v * 0.25}
		case bSnow:
			v := 200 + n*55
			ch, c = []byte{'.', '*', ' ', '\'', '.', ','}[int(n*61)%6], RGB{v * 0.92, v * 0.96, v}
		case bVolcanic:
			v := 40 + n*40
			ch, c = []byte{'.', ',', ':', ' ', '`'}[int(n*19)%5], RGB{v * 1.2, v * 0.9, v * 0.85}
		case bDesert:
			dune := math.Sin(fx/9+fbm(fx/40, fy/20, 91)*6) * 0.5
			v := 140 + n*40 + dune*50
			ch = '.'
			if dune > 0.3 {
				ch = '~'
			} else if n > 0.8 {
				ch = ','
			}
			c = RGB{v * 1.05, v * 0.75, v * 0.4}
		case bSwamp:
			v := 40 + n*40
			c = RGB{v * 0.8, v * 0.9, v * 1.3}
			ch = []byte{',', '.', '"', '\'', ' '}[int(n*17)%5]
		case bCanyon:
			v := 110 + n*50
			ch, c = []byte{'.', ',', ':', '.', '`'}[int(n*13)%5], RGB{v * 1.1, v * 0.55, v * 0.35}
		case bHaunted:
			v := 50 + n*45
			c = RGB{v * 1.2, v * 0.6, v * 1.5}
		case bThunder:
			v := 60 + n*50
			ch, c = []byte{'.', ',', ':', '\'', '`'}[int(n*13)%5], RGB{v * 0.6, v * 1.0, v * 1.2}
		case bCrater:
			v := 90 + n*50
			ch, c = []byte{'.', ',', ':', 'o', '.'}[int(n*13)%5], RGB{v * 1.2, v * 0.6, v * 0.7}
		}
	}
	gch[i] = ch
	gcol[3*i], gcol[3*i+1], gcol[3*i+2] = uint8(cl(c.R)), uint8(cl(c.G)), uint8(cl(c.B))
}

// camino serpenteante entre dos puntos
func road(x0, y0, x1, y1, sd int) {
	n := max(abs(x1-x0), abs(y1-y0)*2)
	if n == 0 {
		return
	}
	px, py := -float64(y1-y0), float64(x1-x0)
	l := math.Hypot(px, py)
	px, py = px/l, py/l
	amp := math.Min(40, float64(n)/12)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		off := amp * math.Sin(t*math.Pi) * (math.Sin(t*7+float64(sd)) + 0.6*math.Sin(t*13+float64(sd)*2.3)) * 0.6
		x := int(float64(x0) + float64(x1-x0)*t + px*off)
		y := int(float64(y0) + float64(y1-y0)*t + py*off*0.5)
		for dx := -1; dx <= 1; dx++ {
			for dy := 0; dy <= 1; dy++ {
				xx, yy := x+dx, y+dy
				if !inb(xx, yy) {
					continue
				}
				switch t0 := ttype[idx(xx, yy)]; {
				case isWaterT(t0) || t0 == tLava || t0 == tBridge:
					ttype[idx(xx, yy)] = tBridge
				case t0 == tWall:
				default:
					ttype[idx(xx, yy)] = tRoad
				}
			}
		}
	}
}

func ellipseDo(cx, cy, rx, ry int, f func(x, y int, d float64)) {
	for y := cy - ry - 1; y <= cy+ry+1; y++ {
		for x := cx - rx - 1; x <= cx+rx+1; x++ {
			dx, dy := float64(x-cx)/float64(rx), float64(y-cy)/float64(ry)
			d := math.Sqrt(dx*dx + dy*dy)
			if d <= 1.0 && inb(x, y) {
				f(x, y, d)
			}
		}
	}
}

func genMap() {
	N := W * H
	ttype, biome, gch, gcol, waterTop = make([]uint8, N), make([]uint8, N), make([]byte, N), make([]uint8, 3*N), make([]int16, N)
	r := rand.New(rand.NewSource(7))

	// biomas: Voronoi con ruido
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx, fy := float64(x), float64(y)
			wx := fx + 300*(fbm(fx/220, fy/110, 101)-0.5)
			wy := fy + 150*(fbm(fx/220, fy/110, 103)-0.5)
			best, bd := uint8(0), 1e18
			for _, s := range seeds {
				dx, dy := wx-float64(s.x), (wy-float64(s.y))*2
				if d := dx*dx + dy*dy; d < bd {
					bd, best = d, s.b
				}
			}
			i := idx(x, y)
			biome[i] = best
			t := tGrass
			e := fbm(fx/22, fy/11, 111)
			switch best {
			case bForest, bDark, bRed:
				t = tForest
				if e > 0.74 {
					t = tRock
				}
			case bSnow:
				t = tSnow
				if e > 0.66 {
					t = tRock
				} else if e < 0.25 {
					t = tIce
				}
			case bVolcanic:
				if e > 0.68 {
					t = tRock
				} else if math.Abs(fbm(fx/30, fy/15, 121)-0.5) < 0.018 {
					t = tLava
				}
			case bCanyon, bThunder:
				if e > 0.6 {
					t = tRock // mesetas
				}
			case bSwamp:
				if fbm(fx/14, fy/7, 131) > 0.6 {
					t = tLake // estanques
				}
			case bPlains:
				if e > 0.8 {
					t = tRock
				}
			case bHaunted:
				if e > 0.75 {
					t = tRock
				}
			}
			ttype[i] = t
		}
	}
	// costa: mar solo al este y sureste, con bahías e islotes
	for y := 0; y < H; y++ {
		coastX[y] = 2890 + int(70*(fbm(float64(y)/90, 3.3, 11)-0.5))
	}
	bays := [][3]int{{2790, 400, 70}, {2900, 640, 40}, {2850, 900, 55}, {2500, 1190, 60}}
	for y := 0; y < H; y++ {
		south := 1170 + int(30*(fbm(float64(x0of(y)), 0, 12)-0.5))
		_ = south
		for x := 0; x < W; x++ {
			sea := x > coastX[y]
			if x > 1700 && float64(y) > 1168+30*(fbm(float64(x)/80, 5.5, 13)-0.5) {
				sea = true
			}
			for _, b := range bays {
				dx, dy := float64(x-b[0])/float64(b[2]), float64(y-b[1])/float64(b[2])*2
				if dx*dx+dy*dy < 1+0.3*(fbm(float64(x)/15, float64(y)/8, 14)-0.5) {
					sea = true
				}
			}
			if sea {
				ttype[idx(x, y)] = tSea
			}
		}
	}
	for k := 0; k < 9; k++ { // islotes
		cx, cy := 2940+r.Intn(50), 80+r.Intn(1000)
		ellipseDo(cx, cy, 8+r.Intn(14), 4+r.Intn(6), func(x, y int, d float64) { setT(x, y, tSand) })
	}
	// playas
	for y := 1; y < H-1; y++ {
		for x := 1; x < W-1; x++ {
			if ttype[idx(x, y)] == tSea {
				continue
			}
			for d := 1; d <= 14; d++ {
				if getT(x+d, y) == tSea || getT(x, y+d/2) == tSea {
					if ttype[idx(x, y)] != tSand {
						ttype[idx(x, y)] = tSand
					}
					break
				}
			}
		}
	}
	// Lago Lunar (inicio) y lagos chicos
	clearAround := func(cx, cy, rx, ry int) {
		ellipseDo(cx, cy, rx, ry, func(x, y int, d float64) {
			if t := ttype[idx(x, y)]; t == tForest || t == tRock {
				ttype[idx(x, y)] = tGrass
			}
		})
	}
	clearAround(lakeCX, lakeCY, lakeRX+40, lakeRY+25)
	lake := func(cx, cy, rx, ry, s int) {
		ellipseDo(cx, cy, rx+6, ry+3, func(x, y int, d float64) {
			if d < 0.9+0.25*(fbm(float64(x)/18, float64(y)/9, s)-0.5) {
				ttype[idx(x, y)] = tLake
			}
		})
	}
	lake(lakeCX, lakeCY, lakeRX, lakeRY, 5)
	for _, l := range [][4]int{{560, 640, 40, 14}, {1750, 470, 35, 12}, {2100, 650, 30, 10}, {1350, 960, 45, 14}, {950, 420, 30, 10}, {2420, 880, 35, 12}, {300, 640, 40, 14}} {
		lake(l[0], l[1], l[2], l[3], l[0])
	}
	// se limpia la zona del foso de la capital
	clearAround((capX0+capX1)/2, (capY0+capY1)/2, (capX1-capX0)/2+40, (capY1-capY0)/2+25)

	// ríos: polilínea con meandros, ancho 4
	river := func(pts [][2]int, w int, s int) {
		for k := 0; k+1 < len(pts); k++ {
			a, b := pts[k], pts[k+1]
			n := max(abs(b[0]-a[0]), abs(b[1]-a[1])*2)
			for i := 0; i <= n; i++ {
				t := float64(i) / float64(max(1, n))
				x := float64(a[0]) + float64(b[0]-a[0])*t
				y := float64(a[1]) + float64(b[1]-a[1])*t
				x += 18 * math.Sin(y/23+float64(s))
				y += 6 * math.Sin(x/37+float64(s))
				for dx := -w / 2; dx <= w/2; dx++ {
					for dy := 0; dy <= 1; dy++ {
						xx, yy := int(x)+dx, int(y)+dy
						if inb(xx, yy) && ttype[idx(xx, yy)] != tLake && ttype[idx(xx, yy)] != tSea {
							ttype[idx(xx, yy)] = tRiver
						}
					}
				}
			}
		}
	}
	river([][2]int{{520, 220}, {600, 380}, {760, 470}, {900, 540}, {lakeCX - 40, lakeCY - 30}}, 4, 1)
	river([][2]int{{lakeCX + 60, lakeCY + 30}, {1200, 820}, {1450, 900}, {1750, 1000}, {2050, 1080}, {2350, 1150}, {2500, 1200}}, 4, 2)
	river([][2]int{{2420, 170}, {2550, 260}, {2650, 350}, {2760, 400}}, 3, 3)
	river([][2]int{{1640, 120}, {1600, 300}, {1700, 420}, {1750, 470}}, 3, 4)
	river([][2]int{{lakeCX, lakeCY - 40}, {lakeCX + 60, 480}, {1180, 300}, {1150, 180}}, 3, 5)

	// caminos: árbol de expansión mínima entre lugares, más algunos enlaces extra
	type edge struct{ a, b int }
	inTree := map[int]bool{0: true}
	var edges []edge
	dist := func(a, b int) float64 {
		return math.Hypot(float64(locs[a].X-locs[b].X), float64(locs[a].Y-locs[b].Y)*2)
	}
	for len(inTree) < len(locs) {
		ba, bb, bd := -1, -1, 1e18
		for a := range inTree {
			for b := range locs {
				if !inTree[b] {
					if d := dist(a, b); d < bd {
						ba, bb, bd = a, b, d
					}
				}
			}
		}
		inTree[bb] = true
		edges = append(edges, edge{ba, bb})
	}
	for a := range locs {
		type nb struct {
			b int
			d float64
		}
		var ns []nb
		for b := range locs {
			if b != a {
				ns = append(ns, nb{b, dist(a, b)})
			}
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].d < ns[j].d })
		if r.Intn(2) == 0 && ns[1].d < 700 {
			edges = append(edges, edge{a, ns[1].b})
		}
	}
	for k, e := range edges {
		road(locs[e.a].X, locs[e.a].Y, locs[e.b].X, locs[e.b].Y, k)
	}

	// glifos de suelo
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			ground(x, y)
		}
	}

	// vegetación y objetos según bioma
	natural := func(t uint8) bool { return t == tForest || t == tGrass || t == tSnow }
	for k := 0; k < 900000; k++ {
		x, y := r.Intn(W-10), r.Intn(H-8)
		i := idx(x, y)
		b, t := biome[i], ttype[i]
		if !(t == tForest || t == tGrass || t == tSnow || t == tSand) {
			continue
		}
		dens := map[uint8]int{bForest: 3, bDark: 2, bRed: 3, bSnow: 9, bPlains: 40, bHaunted: 10, bSwamp: 12, bDesert: 30, bCanyon: 40, bThunder: 25, bVolcanic: 30, bCrater: 60}[b]
		if t != tForest && k%dens != 0 {
			continue
		}
		tint := 0.65 + 0.35*r.Float64()
		trunk := RGB{100 * tint, 70 * tint, 40 * tint}
		if t == tSand {
			if k%6 == 0 && x > 2000 {
				stamp(x, y, palm, func(t uint8) bool { return t == tSand }, -1, func(ch rune, i, j int) RGB {
					if i < 2 {
						return RGB{70, 170, 80}
					}
					return RGB{150, 110, 60}
				})
			}
			continue
		}
		switch b {
		case bForest, bPlains:
			art := pine
			if r.Intn(3) == 0 {
				art = oak
			}
			stamp(x, y, art, natural, -1, func(ch rune, i, j int) RGB {
				if ch == '#' || ch == '|' {
					return trunk
				}
				return RGB{20 * tint, (80 + 60*h2(x+j, y+i, 9)) * tint, 35 * tint}
			})
		case bDark:
			stamp(x, y, pine, natural, -1, func(ch rune, i, j int) RGB {
				if ch == '#' || ch == '|' {
					return trunk
				}
				return RGB{10 * tint, (55 + 40*h2(x+j, y+i, 9)) * tint, 50 * tint}
			})
		case bRed:
			stamp(x, y, oak, natural, -1, func(ch rune, i, j int) RGB {
				if ch == '#' || ch == '|' {
					return trunk
				}
				return RGB{(190 + 60*h2(x+j, y+i, 9)) * tint, (70 + 60*h2(x, y, 4)) * tint, 30 * tint}
			})
		case bSnow:
			stamp(x, y, pine, natural, -1, func(ch rune, i, j int) RGB {
				if ch == '#' || ch == '|' {
					return trunk
				}
				if i%2 == 0 {
					return RGB{230, 240, 250}
				}
				return RGB{40, 100, 80}
			})
		case bHaunted, bVolcanic:
			stamp(x, y, deadTree, natural, -1, solid(RGB{120 * tint, 90 * tint, 110 * tint}))
		case bSwamp:
			stamp(x, y, reeds, natural, -1, solid(RGB{90, 150, 90}))
		case bDesert:
			stamp(x, y, cactus, natural, -1, solid(RGB{60, 150, 70}))
		case bThunder, bCrater:
			stamp(x, y, crystal, natural, -1, solid(RGB{150, 220, 255}))
		case bCanyon:
			setCell(x, y, 'o', RGB{170, 100, 70})
		}
	}

	// estructuras
	for li, l := range locs {
		buildLoc(r, li, l)
	}
	buildLakeHamlet(r)
	stamp(lakeCX+90, lakeCY-60, moonArt, nil, -1, solid(RGB{235, 230, 190}))

	// alto de cada cuerpo de agua, para los reflejos
	for x := 0; x < W; x++ {
		for y := 0; y < H; y++ {
			i := idx(x, y)
			if !isWaterT(ttype[i]) {
				continue
			}
			if y == 0 || !isWaterT(ttype[i-W]) {
				waterTop[i] = int16(y)
			} else {
				waterTop[i] = waterTop[i-W]
			}
		}
	}
}

func x0of(y int) int { return y }

func clearFloor(cx, cy, rx, ry int) {
	ellipseDo(cx, cy, rx, ry, func(x, y int, d float64) {
		i := idx(x, y)
		if isWaterT(ttype[i]) {
			return
		}
		ttype[i] = tFloor
		ground(x, y)
	})
}

func buildLoc(r *rand.Rand, li int, l Loc) {
	switch l.Kind {
	case "capital":
		buildCapital(r)
	case "cave":
		ellipseDo(l.X, l.Y-4, 16, 7, func(x, y int, d float64) {
			if d < 0.95 {
				setT(x, y, tRock)
				ground(x, y)
			}
		})
		clearFloor(l.X, l.Y+3, 8, 2)
		stampC(l.X, l.Y-1, caveMouth, int(tWall), func(ch rune, i, j int) RGB {
			if ch == '#' {
				return RGB{15, 12, 20}
			}
			return RGB{150, 140, 130}
		})
	case "temple", "monastery":
		if l.Kind == "monastery" {
			buildMonastery(r, l)
			break
		}
		clearRect(l.X-40, l.Y-30, l.X+40, l.Y+10, tFloor)
		col := map[string]RGB{"Templo Eterno": {120, 200, 255}, "Templo Oculto": {200, 120, 255}, "Templo Maldito": {150, 60, 70}, "Santuario de Piedra": {170, 150, 110}}[l.Name]
		placeAt(mkTemple(64, 30, col, false), l.X, l.Y+4)
	case "volcano":
		stampC(l.X, l.Y, volcano, int(tWall), func(ch rune, i, j int) RGB {
			if ch == '~' || ch == ',' {
				return RGB{255, 110, 30}
			}
			return RGB{90, 80, 80}
		})
	case "mansion":
		clearRect(l.X-44, l.Y-34, l.X+44, l.Y+10, tFloor)
		placeAt(mkHouse(r, 72, 34, RGB{120, 70, 150}, matStone, 2, ""), l.X, l.Y+4)
	case "hut":
		clearRect(l.X-22, l.Y-26, l.X+22, l.Y+8, tFloor)
		placeAt(mkHouse(r, 30, 24, RGB{170, 140, 70}, matWood, 1, ""), l.X, l.Y+3)
	case "pyramid":
		clearRect(l.X-50, l.Y-32, l.X+50, l.Y+10, tSand)
		placeAt(mkPyramid(92, 34), l.X, l.Y+4)
	case "castle", "skycastle":
		clearRect(l.X-56, l.Y-50, l.X+56, l.Y+12, tFloor)
		roof := RGB{90, 160, 230}
		if l.Kind == "skycastle" {
			roof = RGB{230, 230, 255}
			stampC(l.X-50, l.Y-44, cloud, -1, solid(RGB{220, 225, 240}))
			stampC(l.X+50, l.Y-40, cloud, -1, solid(RGB{220, 225, 240}))
		}
		placeAt(mkCastle(90, 48, roof), l.X, l.Y+4)
	case "arena":
		clearFloor(l.X, l.Y, 46, 18)
		ellipseDo(l.X, l.Y, 40, 15, func(x, y int, d float64) {
			if d > 0.86 && !(abs(y-l.Y) < 2 && x < l.X) {
				setT(x, y, tWall)
				setCell(x, y, []rune{'#', '%', '#', '='}[(x+y)%4], RGB{190, 170, 140})
			} else if d <= 0.86 {
				setT(x, y, tSand)
				ground(x, y)
			}
		})
	case "carnival":
		clearFloor(l.X, l.Y, 40, 15)
		stampC(l.X, l.Y-4, wheel, int(tWall), func(ch rune, i, j int) RGB {
			if ch == 'o' {
				return []RGB{{255, 90, 90}, {255, 220, 80}, {90, 200, 255}}[(i+j)%3]
			}
			return RGB{200, 200, 210}
		})
		tc := []RGB{{255, 80, 120}, {255, 200, 60}, {80, 200, 255}, {160, 255, 120}}
		for k, p := range [][2]int{{-60, -6}, {60, -6}, {-34, 14}, {34, 14}} {
			placeAt(mkTent(28, 16, tc[k], RGB{245, 240, 230}), l.X+p[0], l.Y+p[1])
		}
	case "orc":
		clearFloor(l.X, l.Y, 76, 28)
		ellipseDo(l.X, l.Y, 72, 26, func(x, y int, d float64) {
			if d > 0.9 && !(abs(x-l.X) < 3) {
				setT(x, y, tWall)
				setCell(x, y, 'I', RGB{140, 95, 50})
			}
		})
		for _, p := range [][2]int{{-36, -4}, {0, -8}, {36, -4}, {-22, 18}, {22, 18}} {
			placeAt(mkTent(24, 15, RGB{120, 140, 60}, RGB{140, 100, 60}), l.X+p[0], l.Y+p[1])
		}
	case "mine":
		ellipseDo(l.X, l.Y-6, 26, 9, func(x, y int, d float64) {
			if d < 0.95 {
				setT(x, y, tRock)
				ground(x, y)
			}
		})
		clearFloor(l.X, l.Y+4, 22, 4)
		for _, dx := range []int{-12, 12} {
			stampC(l.X+dx, l.Y-1, caveMouth, int(tWall), func(ch rune, i, j int) RGB {
				if ch == '#' {
					return RGB{15, 12, 20}
				}
				return RGB{150, 130, 110}
			})
		}
		for x := l.X - 20; x <= l.X+20; x++ {
			setCell(x, l.Y+4, '=', RGB{140, 120, 100})
		}
		setCell(l.X-3, l.Y+3, 'o', RGB{180, 160, 120})
		setCell(l.X+3, l.Y+3, 'o', RGB{180, 160, 120})
	case "tower":
		clearRect(l.X-16, l.Y-46, l.X+16, l.Y+6, tFloor)
		placeAt(mkTower(22, 46, RGB{90, 100, 160}), l.X, l.Y+2)
	case "crypt":
		clearRect(l.X-30, l.Y-24, l.X+30, l.Y+8, tFloor)
		placeAt(mkTemple(36, 22, RGB{150, 150, 170}, false), l.X, l.Y+2)
		for k := 0; k < 10; k++ {
			x, y := l.X-28+r.Intn(56), l.Y+4+r.Intn(3)
			setCell(x, y, '+', RGB{150, 150, 160})
			setT(x, y, tWall)
		}
	case "prison":
		clearFloor(l.X, l.Y, 26, 9)
		for y := l.Y - 7; y <= l.Y+5; y++ {
			for x := l.X - 20; x <= l.X+20; x++ {
				if y == l.Y-7 || y == l.Y+5 || x == l.X-20 || x == l.X+20 {
					if !(y == l.Y+5 && abs(x-l.X) < 2) {
						setT(x, y, tWall)
						setCell(x, y, '#', RGB{110, 110, 115})
					}
				} else if (y-l.Y)%4 == 0 && abs(x-l.X) > 3 {
					setT(x, y, tWall)
					setCell(x, y, '|', RGB{150, 150, 160})
				}
			}
		}
	case "village":
		buildVillage(r, l)
	case "cove":
		for k := 0; k < 4; k++ {
			stamp(l.X+30+r.Intn(60), l.Y-15+r.Intn(30), ship, func(t uint8) bool { return t == tSea }, -1, func(ch rune, i, j int) RGB {
				if i < 3 {
					return RGB{230, 230, 220}
				}
				return RGB{130, 90, 50}
			})
		}
		for _, p := range [][2]int{{-50, -14}, {-14, -10}, {-40, 22}} {
			clearRect(l.X+p[0]-16, l.Y+p[1]-22, l.X+p[0]+16, l.Y+p[1]+2, tSand)
			placeAt(mkHouse(r, 28, 22, RGB{170, 140, 70}, matWood, 1, ""), l.X+p[0], l.Y+p[1])
		}
	case "crater":
		ellipseDo(l.X, l.Y, 90, 38, func(x, y int, d float64) {
			i := idx(x, y)
			biome[i] = bCrater
			switch {
			case d > 0.85 && !(abs(y-l.Y) < 3 && x < l.X):
				ttype[i] = tRock
			default:
				ttype[i] = tGrass
			}
			ground(x, y)
		})
		ellipseDo(l.X, l.Y, 8, 3, func(x, y int, d float64) {
			setT(x, y, tWall)
			setCell(x, y, '@', RGB{120, 60, 80})
		})
	case "pass":
		buildCamp(r, l)
	}
	_ = li
}

func zoneAt(x, y int) string {
	if !inb(x, y) {
		return ""
	}
	best, bd := -1, 1e18
	for i, l := range locs {
		d := math.Hypot(float64(x-l.X), float64(y-l.Y)*2)
		if d < float64(l.R)*1.3 && d < bd {
			best, bd = i, d
		}
	}
	if best >= 0 {
		return locs[best].Name
	}
	switch ttype[idx(x, y)] {
	case tSea:
		return "Mar del Este"
	case tRiver:
		return "Río Plateado"
	}
	return biomeNames[biome[idx(x, y)]]
}

func spawnPoint() (int, int) {
	if v := os.Getenv("SPAWN"); v != "" { // para pruebas: SPAWN=x,y
		var x, y int
		if _, err := fmt.Sscanf(v, "%d,%d", &x, &y); err == nil {
			for i := 0; i < 200; i++ {
				xx, yy := x+rand.Intn(30)-15, y+rand.Intn(10)-5
				if !blocked(xx, yy) {
					return xx, yy
				}
			}
			return x, y
		}
	}
	for i := 0; i < 1000; i++ {
		x, y := lakeCX-60+rand.Intn(120), lakeCY-lakeRY-12+rand.Intn(8)
		if !blocked(x, y) && !isWater(x, y) {
			return x, y
		}
	}
	return lakeCX, lakeCY - lakeRY - 8
}

// composición por vista

// compose arma la vista del mundo: terreno, NPCs, enemigos, jugadores y reflejos.
func compose(now time.Time, size byte, vx, vy, vw, vh int) [][]Cell {
	return composeInto(nil, now, size, vx, vy, vw, vh)
}

var layerBuf = map[int]Cell{} // se reutiliza; compose siempre corre con mu tomado

// composeInto dibuja en buf si ya tiene el tamaño correcto (sin reservar memoria por cuadro).
func composeInto(buf [][]Cell, now time.Time, size byte, vx, vy, vw, vh int) [][]Cell {
	t := math.Floor(now.Sub(start).Seconds()*6) / 6 // el agua se anima a 6 fps para mandar menos celdas
	out := buf
	if len(out) != vh || (vh > 0 && len(out[0]) != vw) {
		out = make([][]Cell, vh)
		for i := range out {
			out[i] = make([]Cell, vw)
		}
	}
	layer := layerBuf
	clear(layer)
	rows, cols := figRows(size), figCols(size)
	var waterPlayers []*Player
	drawNPCs(layer, vx, vy, vw, vh, size, now)
	drawEnemies(layer, vx, vy, vw, vh, now)
	for p := range players {
		if p.X < vx-cols-10 || p.X > vx+vw+cols+10 || p.Y < vy-5 || p.Y > vy+vh+rows+60 {
			continue
		}
		if isWater(p.X, p.Y) {
			waterPlayers = append(waterPlayers, p)
			continue
		}
		lean, ov, hurt := poseFor(p, now)
		if size != 'S' {
			lean, ov = 0, nil
		}
		drawFig(size, p.Sprite, p.frame(now), p.Left, p.Prim, p.Sec, func(dx, dy int, ch rune, c RGB) {
			if hurt {
				c = lerp(c, RGB{255, 60, 60}, 0.65)
			}
			layer[(p.Y+dy)*W+p.X+dx+lean] = Cell{ch, c}
		})
		for _, o := range ov {
			layer[(p.Y+o.dy)*W+p.X+o.dx+lean] = Cell{o.ch, o.c}
		}
	}
	src := func(x, y int) (Cell, bool) {
		if !inb(x, y) {
			return Cell{}, false
		}
		if c, ok := layer[y*W+x]; ok {
			return c, true
		}
		if isWaterT(ttype[idx(x, y)]) {
			return Cell{}, false
		}
		return cellAt(x, y), true
	}
	for y := 0; y < vh; y++ {
		wy := vy + y
		for x := 0; x < vw; x++ {
			wx := vx + x
			if !inb(wx, wy) {
				out[y][x] = Cell{' ', RGB{0, 0, 0}}
				continue
			}
			if c, ok := layer[wy*W+wx]; ok {
				out[y][x] = c
				continue
			}
			tt := ttype[idx(wx, wy)]
			if !isWaterT(tt) {
				out[y][x] = cellAt(wx, wy)
				continue
			}
			wb := waterBlue
			if tt == tSea {
				wb = RGB{4, 14, 40}
			}
			top := int(waterTop[idx(wx, wy)])
			d := wy - top
			cell := cellAt(wx, wy)
			if cell.Ch == ' ' {
				cell.C = wb
				if d < 45 {
					shift := int(math.Round(1.6 * math.Sin(float64(wy)*0.9+t*2.2)))
					if c, ok := src(wx+shift, top-1-d); ok && c.Ch != ' ' {
						k := 0.32 * (1 - float64(d)/50)
						cell = Cell{c.Ch, RGB{c.C.R*k + wb.R*(1-k)*3, c.C.G*k + wb.G*(1-k)*3, c.C.B*k + wb.B*(1-k)*3}}
					}
				}
				if cell.Ch == ' ' {
					wv := math.Sin(float64(wx)*0.35 + float64(wy)*2.3 - t*2.5)
					switch tt {
					case tSea:
						wv = math.Sin(float64(wx)*0.5 - t*1.8 + float64(wy)*0.7)
					case tRiver:
						wv = math.Sin(float64(wx)*0.6 + float64(wy)*1.1 - t*4)
					}
					if wv > 0.94 {
						cell = Cell{'~', RGB{wb.R*3 + 60, wb.G*3 + 80, wb.B*3 + 90}}
					}
				}
			}
			out[y][x] = cell
		}
	}
	put := func(x, y int, c Cell) {
		x, y = x-vx, y-vy
		if x >= 0 && y >= 0 && x < vw && y < vh {
			out[y][x] = c
		}
	}
	alive := ripples[:0]
	for _, rp := range ripples {
		age := now.Sub(rp.Start).Seconds()
		if age > 2.6 {
			continue
		}
		alive = append(alive, rp)
		if rp.X < vx-30 || rp.X > vx+vw+30 || rp.Y < vy-20 || rp.Y > vy+vh+20 {
			continue
		}
		rad, fade := age*8, 1-age/2.6
		for a := 0.0; a < 2*math.Pi; a += 0.04 {
			x := rp.X + int(math.Round(math.Cos(a)*rad*2))
			y := rp.Y + int(math.Round(math.Sin(a)*rad))
			if isWater(x, y) {
				ch := '.'
				if int(a*10)%3 == 0 {
					ch = ','
				}
				v := 90 + 165*fade
				put(x, y, Cell{ch, RGB{v * 0.8, v * 0.9, v}})
			}
		}
	}
	ripples = alive
	for _, p := range waterPlayers {
		fr := p.frame(now)
		drawFig(size, p.Sprite, fr, p.Left, p.Prim, p.Sec, func(dx, dy int, ch rune, c RGB) {
			x, y := p.X+dx+int(math.Round(math.Sin(float64(p.Y-dy)*0.9+t*2.2))), p.Y+1-dy
			if isWater(x, y) {
				k := 0.35
				put(x, y, Cell{ch, RGB{c.R*k + waterBlue.R*(1-k)*3, c.G*k + waterBlue.G*(1-k)*3, c.B*k + waterBlue.B*(1-k)*3}})
			}
		})
		lean, ov, hurt := poseFor(p, now)
		if size != 'S' {
			lean, ov = 0, nil
		}
		drawFig(size, p.Sprite, fr, p.Left, p.Prim, p.Sec, func(dx, dy int, ch rune, c RGB) {
			if hurt {
				c = lerp(c, RGB{255, 60, 60}, 0.65)
			}
			put(p.X+dx+lean, p.Y+dy, Cell{ch, c})
		})
		for _, o := range ov {
			put(p.X+o.dx+lean, p.Y+o.dy, Cell{o.ch, o.c})
		}
	}
	return out
}

// colores del mapa
func terrainColor(t, b uint8) RGB {
	switch t {
	case tRock:
		switch b {
		case bCanyon:
			return RGB{150, 70, 45}
		case bThunder:
			return RGB{70, 110, 140}
		case bCrater:
			return RGB{150, 70, 90}
		case bSnow:
			return RGB{180, 185, 200}
		case bVolcanic:
			return RGB{55, 50, 50}
		}
		return RGB{110, 105, 100}
	case tSnow:
		return RGB{225, 230, 240}
	case tIce:
		return RGB{170, 210, 240}
	case tLake, tRiver:
		return RGB{40, 90, 190}
	case tSea:
		return RGB{25, 60, 140}
	case tSand:
		return RGB{215, 195, 135}
	case tRoad, tBridge:
		return RGB{170, 135, 90}
	case tPave:
		return RGB{150, 145, 135}
	case tWall:
		return RGB{190, 185, 180}
	case tFloor:
		return RGB{120, 110, 95}
	case tLava:
		return RGB{240, 90, 30}
	}
	switch b {
	case bPlains:
		return RGB{110, 160, 60}
	case bForest:
		return RGB{40, 120, 50}
	case bDark:
		return RGB{20, 80, 60}
	case bRed:
		return RGB{170, 70, 50}
	case bSnow:
		return RGB{225, 230, 240}
	case bVolcanic:
		return RGB{80, 60, 55}
	case bDesert:
		return RGB{215, 160, 85}
	case bSwamp:
		return RGB{90, 70, 140}
	case bCanyon:
		return RGB{180, 100, 65}
	case bHaunted:
		return RGB{120, 80, 160}
	case bThunder:
		return RGB{70, 140, 160}
	case bCrater:
		return RGB{190, 100, 120}
	}
	return RGB{60, 140, 60}
}

var (
	miniMu    sync.Mutex
	miniCache = map[[2]int][][]RGB{}
)

func minimap(mw, mh int) [][]RGB {
	miniMu.Lock()
	defer miniMu.Unlock()
	k := [2]int{mw, mh}
	if m, ok := miniCache[k]; ok {
		return m
	}
	m := make([][]RGB, mh)
	for y := range m {
		m[y] = make([]RGB, mw)
		for x := range m[y] {
			cnt := map[[2]uint8]int{}
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					wx := min(W-1, x*W/mw+(sx*2+1)*W/(8*mw))
					wy := min(H-1, y*H/mh+(sy*2+1)*H/(8*mh))
					i := idx(wx, wy)
					t := ttype[i]
					w := 1
					if t == tRiver || t == tWall || t == tRoad || t == tPave {
						w = 3
					}
					cnt[[2]uint8{t, biome[i]}] += w
				}
			}
			var best [2]uint8
			bc := -1
			for kk, c := range cnt {
				if c > bc || (c == bc && (kk[0] < best[0] || (kk[0] == best[0] && kk[1] < best[1]))) {
					best, bc = kk, c
				}
			}
			m[y][x] = terrainColor(best[0], best[1])
		}
	}
	miniCache[k] = m
	return m
}

func writeWorldPNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			i := idx(x, y)
			c := terrainColor(ttype[i], biome[i])
			if gch[i] != ' ' && gch[i] != '.' && !isWaterT(ttype[i]) {
				g := cellAt(x, y).C
				c = RGB{(c.R*2 + g.R) / 3, (c.G*2 + g.G) / 3, (c.B*2 + g.B) / 3}
			}
			img.Set(x, y, color.RGBA{uint8(cl(c.R)), uint8(cl(c.G)), uint8(cl(c.B)), 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	j, _ := json.Marshal(locs)
	return os.WriteFile(path+".json", j, 0644)
}
