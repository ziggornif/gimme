# TODO — Refonte du projet Gimme

Les tâches sont ordonnées par priorité et dépendances logiques.
Les tâches de la section suivante ne doivent pas être démarrées tant que les précédentes ne sont pas terminées
(ex : monter Go avant de mettre à jour les dépendances, corriger les bugs avant d'améliorer les tests).

---

## Priorité 1 — Fondations (débloquer tout le reste)

- [x] Monter Go de `1.18` → `1.26` dans `go.mod` et la CI
- [x] Ajouter `gimme.yml` au `.gitignore` (risque de commit accidentel de credentials)

## Priorité 2 — Bugs critiques (corriger avant toute autre modification du code)

- [x] `content-service.go` — `CreatePackage` : erreurs des goroutines d'upload silencieuses → remplacer `sync.WaitGroup` par `errgroup.Group` et propager les erreurs (actuellement retourne `201 Created` même si tous les uploads ont échoué)
- [x] `objectstorage-manager.go` — `RemoveObjects` : `rErr.Err.Error()` panique si `rErr.Err == nil`
- [x] `content-service.go` — `getLatestVersion` : `versions[len(versions)-1]` panique si le slice est vide
- [x] `archive-validator.go` : `Content-Type: application/zip; charset=utf-8` rejeté à tort (comparaison exacte au lieu de `mime.ParseMediaType`)
- [x] `auth-manager.go` — `CreateToken` : erreur de `time.Parse` ignorée avec `_` → expiration silencieusement incorrecte

## Priorité 3 — Mise à jour des dépendances (après montée Go)

- [x] Mettre à jour `gin-gonic/gin` : `v1.8.1` → `v1.11.0`
- [x] Mettre à jour `gin-contrib/cors` : `v1.3.1` → `v1.7.6`
- [x] Mettre à jour `golang-jwt/jwt/v4` : `v4.4.1` → `v4.5.2`
- [x] Mettre à jour `sirupsen/logrus` : `v1.8.1` → `v1.9.4`
- [x] Mettre à jour `prometheus/client_golang` : `v1.12.2` → `v1.23.2`
- [x] Mettre à jour `spf13/viper` : `v1.12.0` → `v1.21.0`
- [x] Mettre à jour `stretchr/testify` : `v1.7.2` → `v1.11.1`
- [x] Mettre à jour `golang.org/x/mod` : `v0.6.0-dev` → `v0.33.0`
- [x] Mettre à jour `minio/minio-go/v7` : `v7.0.28` → `v7.0.98` (conservé en option parallèle à Garage)

## Priorité 4 — Modernisation du code (après mise à jour des dépendances)

- [x] Supprimer `pkg/array` et remplacer `ArrayContains` par `slices.Contains` (Go 1.21+)
- [x] `GimmeError` : renommer `String()` en `Error() string` pour implémenter l'interface `error` standard
- [x] `objectstorage-manager.go` — `RemoveObjects` : remplacer `fmt.Println` par `logrus.Error`
- [x] `objectstorage-manager.go` : remplacer le `context.Background()` hardcodé par propagation du contexte HTTP
- [x] `configs/config.go` — `assertConfigKey` : remplacer la validation par reflection par une validation explicite type-safe

## Priorité 5 — Tests (après correction des bugs et modernisation)

- [x] Séparer les tests unitaires et d'intégration dans `package-controller_test.go` (tag `//go:build integration`)
- [x] Corriger `TestContentService_CreatePackageUploadErr` : le test actuel assert `Nil(err)` en commentant "error is silent here" — corriger après le fix du bug `CreatePackage`
- [x] Ajouter des tests pour les 4 `ErrorKind` manquants dans `business-error_test.go` (`BadRequest`, `Conflict`, `Unauthorized`, `NotImplemented`) et le cas `kind` inconnu
- [x] Ajouter un test pour `getLatestVersion` avec une liste vide
- [x] Ajouter un test pour `Content-Type: application/zip; charset=utf-8` dans `archive-validator_test.go`
- [x] Ajouter un test pour le cas `tokenClaims["exp"] == nil` dans le middleware auth
- [x] Ajouter un test pour `application/octet-stream` dans `archive-validator_test.go` (type valide non testé)
- [x] Corriger `initObjectStorage()` dans `package-controller_test.go` : ne pas ignorer l'erreur avec `_`

## Priorité 6 — CI/CD (après montée Go et mise à jour des dépendances)

- [x] Mettre à jour `actions/checkout@v2` → `@v4` dans `build.yml` et `release.yml`
- [x] Mettre à jour `actions/setup-go@v2` → `@v5` dans `build.yml`
- [x] Mettre à jour `go-version: '^1.18'` → `'^1.26'` dans `build.yml`
- [x] Mettre à jour `wangyoucao577/go-release-action@v1.25` → `@v1.55` dans `release.yml`
- [x] Remplacer `dawidd6/action-get-tag@v1` par la variable native `github.ref_name` dans `release.yml`
- [x] Remplacer le lancement Minio CI par Garage dans `build.yml` (ou proposer les deux)

## Priorité 7 — Dockerfile (indépendant, peut être fait en parallèle de P3+)

- [x] Fixer la version de l'image builder : `golang:1-alpine` → `golang:1.26-alpine`
- [x] Fixer la version de l'image finale : `FROM alpine` → `FROM alpine:3.22`
- [x] Séparer `COPY go.mod go.sum` + `RUN go mod download` avant `COPY . .` pour le cache des dépendances
- [x] Remplacer `ADD . .` par `COPY . .`
- [x] Remplacer `apk update && apk add` par `apk add --no-cache`
- [x] Supprimer le `chmod +x /bin/gimme` inutile
- [x] Ajouter un utilisateur non-root (`adduser -D gimme` + `USER gimme`)
- [x] Ajouter un `HEALTHCHECK`

## Priorité 8 — Storage : Garage HQ (après mise à jour Minio et Dockerfile)

- [x] Vérifier la compatibilité S3 du SDK `minio-go` avec Garage (auth, bucket creation, object ops)
- [x] Ajouter un exemple Docker Compose avec Garage HQ (`dxflrs/garage`) en remplacement de Minio
- [x] Mettre à jour le Docker Compose `with-local-s3` pour proposer les deux options (Minio à jour + Garage)
- [x] Documenter les différences de configuration entre Minio et Garage (voir `examples/deployment/docker-compose/with-garage/README.md`)

## Priorité 9 — Kubernetes (indépendant)

- [x] Supprimer la clé `version:` dépréciée dans les fichiers Docker Compose
- [x] Ajouter `resources` (limits/requests) dans le `Deployment`
- [x] Ajouter les routes `GET /healthz` (liveness) et `GET /readyz` (readiness, vérifie Minio) dans l'application
- [x] Ajouter `livenessProbe` et `readinessProbe` dans le `Deployment` (dépend de la tâche précédente)
- [x] Proposer un exemple d'`Ingress` en complément du `NodePort`
- [x] Proposer un Helm chart de déploiement dans `examples/deployment/helm/` (templates : Deployment, Service, Ingress, ConfigMap, Secret, HPA optionnel via `values.yaml`)
- [x] Publier le chart Helm sur GHCR (OCI) via une GitHub Action (`helm package` + `helm push` sur `ghcr.io/<org>/charts/gimme`) — déclenché sur release

## Priorité 10 — Sécurité & Qualité (indépendant)

- [x] Épingler les versions CDN dans les templates : `redoc@latest` → version fixe, `@picocss/pico@latest` → version fixe
- [x] Compléter `.dockerignore` : ajouter `*.md`, `Makefile`, `gimme.yml`, `.air.toml`, `examples/`
- [x] `auth-controller.go` — `POST /create-token` : retourne `{"error":"EOF"}` si le body est absent et `{"error":"json: cannot unmarshal string into Go value of type api.CreateTokenRequest"}` si le body est une string vide — gérer ces cas avec un `400 Bad Request` explicite plutôt qu'exposer l'erreur interne de décodage

## Priorité 11 — Documentation (en dernier, une fois tout stabilisé)

- [x] Mettre à jour le README : instructions de démarrage local avec Garage + Minio
- [x] Mettre à jour le README : montée de version Go, nouvelles dépendances
- [x] Revoir les exemples de `curl` dans le README (tokens, upload, etc.)
- [x] Vérifier et mettre à jour le contenu statique dans `docs/`
- [x] Revoir les schémas du projet : supprimer `schema.png` (image statique obsolète) et le remplacer par des diagrammes Mermaid intégrés directement dans le README (architecture, flux de données, etc.)
- [x] Mettre à jour `CLAUDE.md` une fois toutes les modifications appliquées

## Priorité 12 — Cache (après stabilisation de la P11)

L'objectif est de proposer deux niveaux de cache indépendants et cumulables :

```
Browser → [Cache externe : proxy/CDN] → [gimme + cache interne] → [S3]
```

### Niveau 1 — Headers HTTP de cache (impact immédiat, zéro dépendance)

- [x] Émettre `Cache-Control: public, max-age=31536000, immutable` sur les fichiers servis avec une version épinglée (`pkg@1.0.0`)
- [x] Émettre `Cache-Control: public, max-age=300` sur les fichiers servis avec une version partielle (`pkg@1.0`) — la résolution peut changer
- [x] Émettre `Cache-Control: no-store` sur les réponses `404` — évite de cacher les absences
- [x] Documenter dans le README comment configurer Nginx/Varnish/Caddy pour exploiter ces headers

### Niveau 2 — Cache interne optionnel (activable via config)

Backend Redis/Valkey (multi-instances, scale-out). L'interface `CacheManager` est conçue pour accueillir facilement un backend mémoire ultérieurement.

```yaml
cache:
  enabled: true
  type: redis       # "redis" supporté ; "memory" prévu
  ttl: 3600         # secondes
  redis_url: redis://localhost:6379  # requis si type: redis
```

- [x] Définir l'interface `CacheManager` (`Get`, `Set`, `Delete`, `DeleteByPrefix`)
- [x] Implémenter le backend Redis/Valkey (ex: `github.com/redis/go-redis/v9`)
- [x] Intégrer le cache dans `content.GetFile` : résolution de version partielle uniquement — le body reste streamé depuis S3 (compromis raisonnable)
- [x] Invalider le cache au `DELETE /packages/:package` (suppression de toutes les entrées `pkg@version/*`)
- [x] Ajouter les tests unitaires (mock `CacheManager`)
- [x] Ajouter Valkey/Redis dans l'exemple Docker Compose `with-garage`
- [x] Documenter la stratégie de cache dans le README (niveaux 1 et 2)

## Priorité 13 — Métriques métier

- [x] Ajouter des métriques applicatives Prometheus : nombre de requêtes par route (counter), latence S3 (histogram), cache hits/misses (counter), nombre de packages uploadés/supprimés (counter)
- [x] Documenter les métriques exposées dans le README

## Priorité 14 — Site de documentation (GitHub Pages)

Site statique déployé sur GitHub Pages, hébergé dans `docs/` à la racine du repo.

**Stack :** HTML/CSS/JS vanilla + libs pragmatiques (Tailwind CDN pour le design, highlight.js pour la coloration syntaxique) — zéro build step.

**Contenu :**

- [ ] Layout global : header, navigation latérale, footer, responsive
- [ ] Page d'accueil : hero accrocheur, points forts du CDN, aperçu de l'architecture
- [ ] Section Quickstart : configuration minimale, premier upload, premier `GET /gimme/...`
- [ ] Section Configuration : tableau complet des options `gimme.yml`, exemples de fichiers
- [ ] Section Deployment : Docker Compose (Garage, Minio, managed S3), Kubernetes/Helm, Systemd
- [ ] Section API Reference : tableau de toutes les routes, exemples `curl` pour chaque route
- [ ] GitHub Actions : workflow `.github/workflows/docs.yml` pour déployer `docs/` sur GitHub Pages
- [ ] Vidéo embarquée (à évaluer) : screencast montrant le déploiement + une utilisation concrète, intégré en section dédiée ou dans le Quickstart

## Priorité 15 — Refonte UI des templates

Coup de frais complet sur les deux templates Go (`templates/`), avec accessibilité RGAA, sémantique HTML correcte et tests E2E.

**État actuel :**
- `index.tmpl` : page d'accueil = simple wrapper ReDoc (Swagger), minimaliste mais fonctionnel
- `package.tmpl` : listing des fichiers d'un package — table basique Pico CSS, taille affichée en octets bruts (illisible), aucune hiérarchie visuelle, pas d'icônes de type de fichier, zéro indication de l'URL de chaque asset

**Travaux :**

- [ ] `package.tmpl` : refonte visuelle complète — design soigné, taille de fichier lisible (Ko/Mo), icône par type de fichier (JS, CSS, image…), URL copiable au clic, breadcrumb `package@version`, responsive
- [ ] `package.tmpl` : accessibilité RGAA — landmarks sémantiques (`<main>`, `<nav>`, `<header>`), attributs `aria-*`, contrastes suffisants, navigation clavier, focus visible
- [ ] `index.tmpl` : revoir la page d'accueil au-delà du simple ReDoc — ajouter un header avec identité visuelle Gimme, liens vers la doc GitHub Pages (P14), avant de charger la spec Swagger
- [ ] Tests E2E Playwright : couvrir la navigation dans un package, la copie d'URL, l'affichage des tailles, et les critères d'accessibilité de base (axe-core via `@axe-core/playwright`)

## Priorité 16 — Helm chart

Le déploiement Kubernetes "à plat" existe (`examples/deployment/kubernetes/`), mais il n'est pas paramétrable et nécessite des modifications manuelles. L'objectif est un chart Helm clé en main, publié sur GHCR en OCI.

**Structure cible : `examples/deployment/helm/gimme/`**

```
Chart.yaml
values.yaml
templates/
  _helpers.tpl
  namespace.yaml
  deployment.yaml
  service.yaml
  ingress.yaml
  configmap.yaml
  secret.yaml
  hpa.yaml            # optionnel, activable via values
  serviceaccount.yaml
```

**`values.yaml` — paramètres clés :**
- image (repository, tag, pullPolicy)
- replicaCount
- resources (requests/limits)
- config (port, secret, admin.user/password, s3.*)
- cache (enabled, type, ttl, redis_url)
- metrics (enabled)
- ingress (enabled, className, host, tls)
- hpa (enabled, minReplicas, maxReplicas, targetCPU)
- serviceAccount (create, name)
- redis (enabled: false par défaut) — **option C** : l'utilisateur fournit son propre `redis_url` OU active `redis.enabled: true` pour déployer un Redis via le sub-chart Bitnami (`Chart.yaml` > `dependencies`). Quand activé, `redis_url` est auto-résolu vers le service K8s interne.

**Tâches :**

- [ ] Créer le chart Helm dans `examples/deployment/helm/gimme/` avec tous les templates listés ci-dessus
- [ ] Gérer les credentials sensibles dans un `Secret` K8s distinct du `ConfigMap` (secret JWT, admin password, S3 secret)
- [ ] Valider le chart avec `helm lint` et `helm template`
- [ ] Ajouter un `README.md` dans le chart avec les instructions d'installation (`helm install`, override via `--set` ou `-f`)
- [ ] GitHub Actions : publier le chart sur GHCR en OCI (`helm package` + `helm push ghcr.io/<org>/charts/gimme`) déclenché sur release
