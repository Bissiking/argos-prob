# Argos Prob

**Argos Prob** est l'agent hôte léger et multiplateforme d'Argos. Il se connecte
au **master Argos** pour y être supervisé : le master accepte ou refuse son
association, puis l'agent pousse son snapshot à l'intervalle consigné.

L'agent reste fonctionnel quand les composants optionnels (Docker, Proxmox,
gestionnaire de services) sont absents.

## Cibles supportées

- Windows 10 / 11 et Windows Server
- Linux (Debian, Ubuntu, …)
- Hôtes Proxmox VE
- macOS Intel et Apple Silicon

Argos Prob lui-même ne requiert pas Docker.

## Démarrage rapide

```bash
go build -o argos-prob ./cmd/argos-prob
./argos-prob init          # renseigner l'URL du master et le jeton d'invitation
./argos-prob run           # attendre l'approbation puis pousser les métriques
```

`init` accepte aussi les flags `--url` et `--token` :

```bash
./argos-prob init --url https://argos.example.net --token AR-xxxxxxxx
```

## Flux d'association

1. Le **master** génère une invitation (`Paramètres → Serveurs → Inviter un agent`) :
   une URL de master et un jeton `AR-…` (valable 7 jours, à usage unique).
2. Sur la machine cible, `argos-prob init` demande **URL du master** et **jeton**,
   puis crée une **demande d'association** (hostname + IP) sur le master.
3. Le master **accepte ou refuse** la demande.
4. Acceptée, `argos-prob run` récupère la consigne d'intervalle
   (`PUSH_INTERVAL_MS` du master) et pousse un snapshot toutes les N ms via
   `POST /api/v1/agents/metrics`. Sans poussée pendant `OFFLINE_AFTER_MS`, le
   master marque l'agent `degraded` puis `offline`.
5. Refusée, l'agent cesse et demande une nouvelle invitation.

## Commandes

| Commande | Rôle |
| --- | --- |
| `init` | Associe l'hôte au master (URL + jeton), écrit la configuration |
| `run` | Attend l'approbation puis pousse les métriques en boucle |
| `status` | Affiche l'inventaire local (snapshot) au format JSON |
| `doctor` | Diagnostic local |
| `version` | Version |

Le mode **active** (pull, exposé sur `/api/v1/snapshot`) reste disponible en
configurant `"mode": "active"` : le master interroge alors l'agent directement.

## Configuration

Chemins par défaut :

- Linux root : `/etc/argos-prob/config.json`
- Linux/macOS utilisateur : le répertoire de configuration utilisateur
- Windows : `%ProgramData%\ArgosProb\config.json`

Pour le développement, surcharger le chemin avec `ARGOS_PROB_CONFIG`.

```json
{
  "agent_id": "<identité persistante>",
  "name": "<hostname>",
  "mode": "passive",
  "endpoint": "https://argos.example.net",
  "token": "AR-…"
}
```

## Snapshot transmis

Le snapshot correspond au contrat `AgentSnapshot` du master : CPU (usage, load,
cœurs), mémoire + swap, volumes de stockage, interfaces réseau, services
(systemd), conteneurs Docker et VM/CT Proxmox.

## Principes de sécurité

- Pas d'exécution shell arbitraire à distance.
- Docker et Proxmox sont des capacités optionnelles, pas des dépendances.
- Un fournisseur absent ne doit pas empêcher l'inventaire hôte de fonctionner.
- Les actions à distance utiliseront une liste explicite.
- L'inventaire en lecture seule précède toute administration à distance.

## Portée actuelle

Version `0.1.0-dev` fournit :

- identité d'agent persistante (agent_id, hostname)
- OS / noyau / architecture / IP
- CPU : usage %, load 1/5/15, nombre de cœurs
- mémoire et swap
- volumes de stockage (taille, utilisé, disponible, usage %)
- interfaces réseau (adresses, état, débit, octets RX/TX)
- services systemd (Linux), conteneurs Docker, VM/CT Proxmox
- détection des capacités (Docker, systemd, Proxmox, Windows Services, launchd)
- association push avec demandes approuvées/refusées par le master
- diagnostic local et inventaire JSON

Prochaines étapes : installation en service natif, plus de détails
d'inventaire, isolation par fournisseur, logs structurés.