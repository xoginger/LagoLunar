# Arquitectura

Lago Lunar es un solo proceso. Cada conexión SSH es un programa de bubbletea
con su propio `model`, y todos comparten el mundo en memoria, protegido por el
mutex global `mu` (jugadores, enemigos, efectos, NPCs, chat y equipos).

## Generación del mundo

`genMap` (world.go) arma el continente de 3000 x 1200 celdas a partir de una
semilla fija, así que el mapa es igual en cada arranque:

1. Biomas con un Voronoi deformado por ruido.
2. Costa al este y sureste, playas e islotes.
3. El Lago Lunar al centro, lagos chicos y ríos con meandros.
4. Caminos: un árbol de expansión mínima entre los lugares más algunos enlaces
   extra para que haya ciclos.
5. Suelo, vegetación y objetos según el bioma, y al final las estructuras
   (capital con murallas, pirámide, pueblos).

Cada celda guarda tipo, bioma, glifo y color en 8 bits, unos 25 MB en total.
Al arrancar también se colocan las zonas seguras, los NPCs y hasta 900
enemigos, más fuertes mientras más lejos del lago.

## Dibujo de un cuadro

`gameCanvas` (main.go) calcula la cámara y llama a `composeInto`, que junta en
una capa los NPCs, enemigos y jugadores visibles, y luego llena la vista con el
terreno. El agua refleja lo que tiene encima y se anima a 6 fps. Encima van
los efectos, el HUD, las ventanas y el chat.

La cámara tiene zona muerta: no se mueve mientras el jugador anda cerca del
centro. Así, al caminar, solo cambian las celdas del sprite.

## Renderer por diferencias

Dentro del juego bubbletea ya no dibuja: `View` devuelve un texto fijo y los
cuadros los manda `rawRenderer` (render.go), una goroutine por sesión:

- Convierte el canvas a códigos de color según el modo del terminal
  (truecolor, 256 o 16) usando tablas precalculadas.
- Lo compara con el cuadro anterior y manda solo las celdas que cambiaron,
  moviendo el cursor y repitiendo el color solo cuando hace falta. Si dos
  cambios en la misma fila están cerca, reescribe el hueco en lugar de mover
  el cursor.
- Envía como máximo 20 cuadros por segundo. Si la conexión va lenta, se queda
  solo con el cuadro más nuevo y descarta los demás, así la entrada nunca se
  bloquea.
- Al entrar al juego o cambiar el tamaño de la ventana redibuja todo durante
  un momento.

La salida de bubbletea y la del renderer comparten un `lockedW` para no
mezclar bytes.

## Combate y simulación

`simLoop` corre `simStep` 10 veces por segundo con `mu` tomado:

- regeneración de vida y maná,
- IA de enemigos: eligen a un jugador cercano fuera de zona segura, lo siguen y
  atacan con su propio tiempo de espera,
- reaparición de enemigos muertos,
- limpieza de efectos vencidos.

Los ataques del jugador (`basicAttack`, `special` en combat.go) se resuelven
al momento en el servidor: daño, XP, oro, botín y misiones. Lo visual va
aparte en anim.go: la pose del personaje, el proyectil que sale de la mano y el
número de daño se programan con un retraso para que coincidan con el impacto.
Todos los jugadores ven las mismas animaciones porque son efectos del mundo.

Al morir, el jugador reaparece en el Lago Lunar y pierde un poco de XP y oro.

## Cuentas y datos

Las cuentas están en `accounts.json` (contraseñas con bcrypt) y se guardan de
forma atómica. Hay límite de intentos de login por usuario y por IP. El
progreso se guarda cada tanto y al salir. Las bitácoras de sesiones y de admin
son archivos JSON por línea, y la CLI `admin` los lee directo.
