# Idea pendiente: control remoto por sesión (web + escritorio)

Estado: **idea guardada para retomar más adelante.** No hay nada implementado.

## Objetivo

Que cada Buddy app tenga, además de la app de escritorio, una versión web
sincronizada con ella, al estilo de Discord: ambas muestran lo mismo y lo que
se cambia en una se refleja en la otra.

## Planteo

- La **app de escritorio es el motor**: maneja el hardware (interfaces DMX,
  placas de sonido, salidas de video) y es la fuente de verdad del estado.
- La **app web es la misma interfaz** conectada a un motor que corre en otro
  equipo. No es una copia independiente: un navegador remoto no puede tocar
  el hardware.
- **Sesión:** el escritorio mantiene una conexión WebSocket y publica su
  estado; la web se une con un código de emparejamiento o una cuenta. Cada
  cambio va al motor, que lo aplica y lo difunde a todos los conectados.

## Alcances

| | Red local | Por internet |
|---|---|---|
| Conexión | La web entra directo a la app de escritorio | Ambas se conectan a un servidor intermedio |
| Requiere | Un PIN y abrir el panel a la red | Servidor, cuentas o códigos de sesión, hosting |
| Latencia | Mínima | Depende de internet |
| Sin internet | Funciona | No funciona |

## Límites a respetar

- Por la sesión viajan **órdenes y estado**, no la señal: DMX, audio y video
  en tiempo real se quedan en la máquina local.
- **Seguridad** desde el inicio: autenticación y permisos (quien entra a la
  sesión controla un show en vivo).
- Si se cae la conexión, **el show sigue** en el escritorio.
- Vercel no mantiene WebSockets permanentes: el servidor intermedio iría en
  otro servicio.

## Recomendación acordada

1. Empezar por **red local**; internet en una segunda etapa.
2. Definir **un único protocolo de sesión** para todas las Buddy apps; es la
   misma base para conectarlas entre sí más adelante.
3. Usar **LJ Buddy ArtNet Translator como prototipo**: pasar su panel a
   WebSocket y permitir abrirlo desde otro dispositivo de la red con un PIN.
   La carpeta `artnet-translator/web-example/` se conserva como punto de
   partida.
