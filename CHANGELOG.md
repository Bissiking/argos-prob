# Notes de version

## 1.5.1 — 2026-09-25

- Corrige la version intégrée aux binaires : la variable Go peut désormais être remplacée par le linker lors de la construction des paquets.
- La commande `version`, les snapshots et le endpoint de santé annoncent la même version de compilation.
- La détection de version du Makefile utilise `awk`, compatible Linux et macOS.
- Ajoute un test qui compile un binaire avec une version injectée et vérifie la valeur annoncée.

Après installation d’un paquet reconstruit, redémarrer le processus agent pour transmettre la nouvelle version au Master. Le protocole d’échange reste inchangé.
