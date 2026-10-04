# Publication LUMA Store — Argos Prob 1.5.2

La version 1.5.2 corrige la synchronisation des autorisations en cours d’exécution. Construire les artefacts Linux avec Python 3 et Go :

```sh
go test ./...
python packaging/build-linux.py
```

## Artefacts Linux produits

| Plateforme | Architecture Store | Format | Fichier |
|---|---|---|---|
| Linux | x64 | deb | `dist/packages/argos-prob_1.5.2_amd64.deb` |
| Linux | arm64 | deb | `dist/packages/argos-prob_1.5.2_arm64.deb` |
| Linux | x64 | tar.gz | `dist/argos-prob-1.5.2-linux-amd64.tar.gz` |
| Linux | arm64 | tar.gz | `dist/argos-prob-1.5.2-linux-arm64.tar.gz` |

Les paquets DEB sont les artefacts principaux pour la mise à jour depuis Argos. Les TAR.GZ permettent une installation manuelle. Le manifeste `dist/release-1.5.2-linux.json` précise les formats, architectures, chemins et SHA-256 ; `dist/SHA256SUMS-1.5.2-linux.txt` permet de contrôler les téléchargements.

Publier les DEB avec la version `1.5.2` et le canal `stable`, en sélectionnant l’architecture correcte. La construction locale ne publie aucun paquet. Les archives et installateurs Windows/macOS restent à reconstruire avec les cibles correspondantes du Makefile et leurs outils ; ils ne sont pas produits par `build-linux.py`.

Le paquet conserve la configuration et l’association existantes, puis redémarre l’agent après mise à jour. Une première installation nécessite `sudo argos-prob init` puis `sudo systemctl enable --now argos-prob`.
