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
- [ ] Ajouter `resources` (limits/requests) dans le `Deployment`
- [ ] Ajouter les routes `GET /healthz` (liveness) et `GET /readyz` (readiness, vérifie Minio) dans l'application
- [ ] Ajouter `livenessProbe` et `readinessProbe` dans le `Deployment` (dépend de la tâche précédente)
- [ ] Proposer un exemple d'`Ingress` en complément du `NodePort`
- [ ] Documenter les options HPA et PodDisruptionBudget dans le README Kubernetes

## Priorité 10 — Sécurité & Qualité (indépendant)

- [x] Épingler les versions CDN dans les templates : `redoc@latest` → version fixe, `@picocss/pico@latest` → version fixe
- [x] Compléter `.dockerignore` : ajouter `*.md`, `Makefile`, `gimme.yml`, `.air.toml`, `examples/`
- [x] `auth-controller.go` — `POST /create-token` : retourne `{"error":"EOF"}` si le body est absent et `{"error":"json: cannot unmarshal string into Go value of type api.CreateTokenRequest"}` si le body est une string vide — gérer ces cas avec un `400 Bad Request` explicite plutôt qu'exposer l'erreur interne de décodage

## Priorité 11 — Documentation (en dernier, une fois tout stabilisé)

- [ ] Mettre à jour le README : instructions de démarrage local avec Garage + Minio
- [ ] Mettre à jour le README : montée de version Go, nouvelles dépendances
- [ ] Revoir les exemples de `curl` dans le README (tokens, upload, etc.)
- [ ] Vérifier et mettre à jour le contenu statique dans `docs/`
- [ ] Revoir les schémas du projet : supprimer `schema.png` (image statique obsolète) et le remplacer par des diagrammes Mermaid intégrés directement dans le README (architecture, flux de données, etc.)
- [ ] Mettre à jour `CLAUDE.md` une fois toutes les modifications appliquées
