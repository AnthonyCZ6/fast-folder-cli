# Política de seguridad

## Versiones con soporte

Solo la versión más reciente recibe correcciones de seguridad. Si usas una anterior,
actualiza antes de reportar: vuelve a ejecutar el asistente de instalación o el instalador
de un solo comando (consulta el [README](README.md)).

| Versión | Soporte |
| --- | --- |
| La última publicada | ✅ |
| Anteriores | ❌ |

## Cómo reportar una vulnerabilidad

**No abras un issue público.** Repórtala en privado desde la pestaña **Security** del
repositorio, con
[**Report a vulnerability**](https://github.com/AnthonyCZ6/fast-folder-cli/security/advisories/new).
Solo la ve el mantenedor.

Si puedes, incluye:

- La versión (`fast-folder-cli --version`) y la de Windows.
- Los pasos para reproducirla y qué consigue quien la aprovecha.
- Una idea de cómo corregirla, si la tienes.

## Qué esperar

- Intento responder en menos de 7 días.
- Si se confirma, se corrige en una versión nueva y se publica un aviso de seguridad
  (*GitHub Security Advisory*) que menciona a quien la reportó, si así lo quiere.

## Alcance

Cuenta como vulnerabilidad, por ejemplo:

- Que el asistente (`fast-folder-cli-setup.exe`) o `install.ps1` instalen archivos que no
  corresponden a la versión publicada, o que `install.ps1` se salte la comprobación SHA-256.
- Que `fcd`, el menú contextual o las acciones del modo interactivo (abrir VS Code, una
  terminal o el Explorador) ejecuten comandos inyectados en el nombre de una carpeta, por
  ejemplo con `&` o comillas.
- Que el programa lea o envíe datos más allá de lo que describe la
  [política de privacidad](README.md#code-signing-policy).

No cuentan:

- Que los ejecutables no estén firmados, ni los avisos de SmartScreen o Smart App Control
  (consulta [Si Windows bloquea el programa](README.md#si-windows-bloquea-el-programa)).
- Problemas que requieren que el atacante ya controle tu cuenta de Windows.
