# LJ Buddy ArtNet Translator

Complemento de [LJ Buddy](../README.md). Puente **Art-Net → DMX** para interfaces USB. Recibe universos Art-Net por la
red y los envía por una o varias interfaces DMX conectadas al equipo, con un
universo por salida. Detecta las interfaces solo, también al enchufarlas con
el programa abierto.

## Interfaces compatibles

| Interfaz | Universos | Cómo se detecta |
|---|---|---|
| Enttec Open DMX USB y clones FTDI (FT232R/FT232H…) | 1 | USB FTDI que no responde al sondeo "Pro" |
| Enttec DMX USB Pro y compatibles (mismo protocolo) | 1 | Responde al sondeo con su número de serie |
| Enttec DMX USB Pro Mk2 | 1 (solo el puerto 1) | Ídem |
| DMXking ultraDMX Pro | 2 | Se identifica como DMXking |
| Chips FTDI de varios canales (FT2232/FT4232) | 1 por canal | Cada canal es un puerto serie |
| Otros adaptadores serie USB-RS485 (CH340, CP210x…) | 1 | Se listan aparte; botón **Usar como interfaz DMX** |

No compatibles: interfaces que no son un puerto serie (uDMX/Anyma, DMXIS,
Enttec Open DMX *Ethernet*) y el puerto 2 del DMX USB Pro Mk2, que requiere
una clave de API que Enttec entrega a cada desarrollador.

## Instalación en Windows

1. Descarga y ejecuta `LJ-Buddy-ArtNet-Translator-Setup-<versión>.exe`.
   Como el instalador no está firmado, Windows SmartScreen puede avisar:
   **Más información → Ejecutar de todas formas**.
2. Elige los componentes (acceso directo, iniciar con Windows). El instalador
   crea la regla de firewall para Art-Net (UDP 6454).
3. Conecta la interfaz DMX. Windows 10/11 instala el driver FTDI
   automáticamente con Windows Update. Si no aparece como puerto COM,
   instala el driver CDM desde <https://ftdichip.com/drivers/vcp-drivers/>.

También hay una versión portable (`.zip`) que no necesita instalación.

## Uso

Al abrirlo se muestra la ventana del programa y aparece un icono en la
bandeja del sistema. Cerrar la ventana no detiene el puente: sigue
funcionando en la bandeja. Para volver a verla, usa **Abrir** en el menú del
icono o ejecuta el programa otra vez; para detenerlo, **Salir**.

La ventana usa WebView2, que viene con Windows 10 y 11. Si no está
instalado, el panel se abre en el navegador (<http://127.0.0.1:6455/>), igual
que en Linux.

- **Entrada Art-Net**: interfaz de red donde escuchar y, opcionalmente, la
  **IP de origen**: solo se aceptan datos de ese emisor. Los emisores
  detectados aparecen listados; con un clic se elige uno.
- **Interfaces DMX**: cada interfaz detectada con su tipo, puerto y universo.
  A cada salida nueva se le asigna el primer universo libre (a partir del
  "primer universo para autoasignar"). El universo se puede cambiar en
  cualquier momento y la asignación se recuerda por número de serie, aunque
  cambie el puerto COM.
  - **Probar salida** envía todos los canales al 100 % para comprobar el
    cableado sin Art-Net.
  - **Tipo** permite forzar el protocolo si la autodetección no acierta.
- **Universos recibidos** y **Monitor de canales** muestran qué llega.

Los universos usan el Port-Address de Art-Net (0–32767). Si tu software
numera desde 1, su "universo 1" suele ser el 0 de Art-Net.

El complemento responde a ArtPoll, así que aparece como nodo en el software de
iluminación. Si se pierde la señal Art-Net se mantiene el último valor.

La configuración se guarda en `%AppData%\LJBuddyArtNetTranslator\config.json` y el registro
en `lj-buddy-artnet-translator.log` de la misma carpeta.

### Linux

```sh
tar xzf LJ-Buddy-ArtNet-Translator-<versión>-linux-amd64.tar.gz && ./lj-buddy-artnet-translator
```

El usuario necesita acceso a los puertos serie (`sudo usermod -aG dialout $USER`).

### Opciones

```
-http 127.0.0.1:6455   dirección interna del panel
-hidden                iniciar sin abrir la ventana (queda en la bandeja)
-sim N                 N interfaces simuladas, para probar sin hardware
-config ruta           archivo de configuración
```

## Compilar

Requiere Go 1.25+ y, para el instalador, NSIS (`makensis`).

```sh
cd artnet-translator
./build.sh            # deja todo en artnet-translator/dist/
```

Para incluir el driver FTDI en el instalador, guarda el instalador CDM de
FTDI como `artnet-translator/installer/drivers/CDM-Setup.exe` antes de compilar.

Cada push ejecuta la misma compilación en GitHub Actions y publica los
entregables como artefactos; una etiqueta `v*` crea además una Release.

## Cómo funciona

- `internal/artnet`: decodifica ArtDmx y responde ArtPoll.
- `internal/dmx`: drivers. Open DMX genera la trama completa a 250000 baudios
  8N2 con BREAK por la línea serie (~36 Hz); Pro envía mensajes
  `0x7E … 0xE7` y la interfaz genera la señal.
- `internal/engine`: enumera puertos cada 2 s, sondea, asigna universos y
  reparte los datos.
- `internal/web`: panel local (solo accesible desde el propio equipo). En
  Windows se muestra en una ventana propia con WebView2 (`ui_windows.go`).

## Licencia

MIT. El driver FTDI y las marcas Enttec y DMXking pertenecen a sus dueños.

## Demo en línea

El panel se puede probar con datos simulados, sin instalar nada, en
<https://lj-buddy.vercel.app/>.
`web-example/build.sh` genera la demo en `_site/` a partir del panel real
anteponiéndole un motor simulado (`web-example/mock.js`), y `vercel.json` hace que
Vercel la construya sola al importar el repositorio con `artnet-translator`
como directorio raíz.

La carpeta `web-example/` se conserva tal cual como base para un futuro
control remoto del complemento por WebSocket.
