# Notes de version

## 1.5.2 — 2026-10-04

- En mode push, relit les autorisations et la cadence du master à chaque cycle, avant l’inventaire et les commandes. Les modifications s’appliquent sans redémarrer l’agent.
- Applique aussi une liste d’autorisations entièrement vide pour révoquer tous les contrôles.
- Suspend les commandes si la configuration du master n’a pas pu être actualisée.
- Ajoute des tests HTTP de synchronisation en cours d’exécution, de révocation, de politique invalide et de suspension des commandes.

- Prépare des paquets Linux DEB et TAR.GZ x64/ARM64 avec un constructeur Python utilisable aussi depuis Windows, un manifeste et des SHA-256.
- Conserve la configuration et le jeton lors d’une mise à jour Debian ; les paquets ne livrent plus de configuration factice.
- Autorise l’écriture dans `/etc/argos-prob` sous systemd et redémarre l’agent après la mise à jour pour charger le nouveau binaire.

## 1.5.1 — 2026-09-25

- Corrige la version intégrée aux binaires : la variable Go peut désormais être remplacée par le linker lors de la construction des paquets.
- La commande `version`, les snapshots et le endpoint de santé annoncent la même version de compilation.
- La détection de version du Makefile utilise `awk`, compatible Linux et macOS.
- Ajoute un test qui compile un binaire avec une version injectée et vérifie la valeur annoncée.

Après installation d’un paquet reconstruit, redémarrer le processus agent pour transmettre la nouvelle version au Master. Le protocole d’échange reste inchangé.
