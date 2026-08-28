# Publication LUMA Store — Argos Prob 1.3.0

Ce document répertorie tous les artefacts produits dans `dist/` et indique les
valeurs à sélectionner dans le formulaire LUMA Store.

## Artefacts publiables

| Plateforme | Architecture | Format | Version système min. | Fichier | Usage conseillé |
|---|---|---|---|---|---|
| Windows | x64 | msi | Windows 10 / Windows Server 2016 | `dist/packages/argos-prob-1.3.0-windows-amd64.msi` | Installation standard — recommandé |
| Windows | x64 | exe | Windows 10 / Windows Server 2016 | `dist/argos-prob-windows-amd64.exe` | Binaire portable ou future mise à jour |
| Windows | x64 | zip | Windows 10 / Windows Server 2016 | `dist/argos-prob-1.3.0-windows-amd64.zip` | Distribution portable manuelle |
| macOS | x64 | tar.gz | macOS 12 Monterey | `dist/argos-prob-1.3.0-darwin-amd64.tar.gz` | Distribution Intel — recommandé |
| macOS | ARM64 | tar.gz | macOS 12 Monterey | `dist/argos-prob-1.3.0-darwin-arm64.tar.gz` | Distribution Apple Silicon — recommandé |
| Linux | x64 | deb | Debian 11+ / Ubuntu 20.04+ | `dist/packages/argos-prob_1.3.0_amd64.deb` | Installation Debian/Ubuntu — recommandé |
| Linux | ARM64 | deb | Debian 11+ / Ubuntu 20.04+ | `dist/packages/argos-prob_1.3.0_arm64.deb` | Installation Debian/Ubuntu ARM — recommandé |
| Linux | x64 | tar.gz | Noyau Linux 3.2+ | `dist/argos-prob-1.3.0-linux-amd64.tar.gz` | Distribution Linux générique |
| Linux | ARM64 | tar.gz | Noyau Linux 3.2+ | `dist/argos-prob-1.3.0-linux-arm64.tar.gz` | Distribution Linux générique ARM |

## Sélection recommandée pour une publication simple

Pour ne proposer qu'un téléchargement principal par système et architecture :

| Plateforme | Architecture | Artefact principal |
|---|---|---|
| Windows | x64 | `dist/packages/argos-prob-1.3.0-windows-amd64.msi` |
| macOS | x64 | `dist/argos-prob-1.3.0-darwin-amd64.tar.gz` |
| macOS | ARM64 | `dist/argos-prob-1.3.0-darwin-arm64.tar.gz` |
| Linux | x64 | `dist/packages/argos-prob_1.3.0_amd64.deb` |
| Linux | ARM64 | `dist/packages/argos-prob_1.3.0_arm64.deb` |

Le Store ne départage pas encore plusieurs formats ayant la même plateforme,
la même architecture, la même version et le même canal. Pour les mises à jour
automatiques, ne publier qu'un artefact principal par combinaison. Les formats
portables peuvent être conservés pour un téléchargement manuel lorsque l'API
saura filtrer sur le type de paquet.

## Fichiers techniques présents dans `dist/`

Ces fichiers participent à la construction des archives et des installateurs.
Ils ne doivent pas être envoyés directement au Store, à l'exception de l'EXE
Windows si une distribution portable est volontairement proposée.

| Fichier ou dossier | Rôle |
|---|---|
| `dist/argos-prob-windows-amd64.exe` | Binaire Windows brut ; publiable uniquement avec le format `exe` |
| `dist/argos-prob-darwin-amd64` | Binaire macOS Intel contenu dans l'archive TAR.GZ |
| `dist/argos-prob-darwin-arm64` | Binaire macOS Apple Silicon contenu dans l'archive TAR.GZ |
| `dist/argos-prob-linux-amd64` | Binaire Linux x64 contenu dans les archives et paquets |
| `dist/argos-prob-linux-arm64` | Binaire Linux ARM64 contenu dans les archives et paquets |
| `dist/msi-staging/bin/argos-prob.exe` | Copie de travail intégrée au MSI |
| `dist/msi-staging/bin/config.json` | Configuration d'exemple intégrée au MSI |
| `dist/msi-staging/installer.wxs` | Source WiX générée pour construire le MSI |
| `dist/argos-prob_1.3.0_amd64/` | Arborescence temporaire du paquet DEB x64 |
| `dist/argos-prob_1.3.0_arm64/` | Arborescence temporaire du paquet DEB ARM64 |

## Cibles non disponibles

- Windows ARM64 : aucun build n'est déclaré actuellement.
- Android : Argos Prob ne cible pas Android.
- DMG, AppImage et RPM : aucun artefact final correspondant n'est actuellement
  présent dans `dist/`.
- Architecture universelle : aucun binaire multiarchitecture n'est produit ;
  sélectionner explicitement `x64` ou `ARM64`.

## Commandes de reconstruction

```bash
mkdir -p dist/packages
go test ./...
make archive
make package-deb
make package-msi
```

Ne pas utiliser `make dist` tant que sa dépendance inexistante `archive-all`
n'a pas été corrigée.
