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

- [x] Layout global : header, navigation latérale, footer, responsive
- [x] Page d'accueil : hero accrocheur, points forts du CDN, aperçu de l'architecture
- [x] Section Quickstart : configuration minimale, premier upload, premier `GET /gimme/...`
- [x] Section Configuration : tableau complet des options `gimme.yml`, exemples de fichiers
- [x] Section Deployment : Docker Compose (Garage, Minio, managed S3), Kubernetes/Helm, Systemd
- [x] Section API Reference : tableau de toutes les routes, exemples `curl` pour chaque route
- [x] GitHub Actions : workflow `.github/workflows/docs.yml` pour déployer `docs/` sur GitHub Pages
- [ ] ~~Vidéo embarquée~~ — **déplacé en P18**

## Priorité 15 — Refonte UI des templates

Coup de frais complet sur les deux templates Go (`templates/`), avec accessibilité RGAA, sémantique HTML correcte et tests E2E.

**État actuel :**
- `index.tmpl` : page d'accueil = simple wrapper ReDoc (Swagger), minimaliste mais fonctionnel
- `package.tmpl` : listing des fichiers d'un package — table basique Pico CSS, taille affichée en octets bruts (illisible), aucune hiérarchie visuelle, pas d'icônes de type de fichier, zéro indication de l'URL de chaque asset

**Travaux :**

- [x] `package.tmpl` : refonte visuelle complète — design soigné, taille de fichier lisible (Ko/Mo), icône par type de fichier (JS, CSS, image…), URL copiable au clic, breadcrumb `package@version`, responsive
- [x] `package.tmpl` : accessibilité RGAA — landmarks sémantiques (`<main>`, `<nav>`, `<header>`), attributs `aria-*`, contrastes suffisants, navigation clavier, focus visible
- [x] `index.tmpl` : revoir la page d'accueil au-delà du simple ReDoc — ajouter un header avec identité visuelle Gimme, liens vers la doc GitHub Pages (P14), avant de charger la spec Swagger
- [ ] ~~Tests E2E Playwright~~ — **déplacé en P18**

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

- [x] Créer le chart Helm dans `examples/deployment/helm/gimme/` avec tous les templates listés ci-dessus
- [x] Gérer les credentials sensibles dans un `Secret` K8s distinct du `ConfigMap` (secret JWT, admin password, S3 secret)
- [x] Valider le chart avec `helm lint` et `helm template`
- [x] Ajouter un `README.md` dans le chart avec les instructions d'installation (`helm install`, override via `--set` ou `-f`)
- [x] GitHub Actions : publier le chart sur GHCR en OCI (`helm package` + `helm push ghcr.io/<org>/charts/gimme`) déclenché sur release

## Priorité 17 — Authentification & gestion des tokens

L'objectif est de remplacer le système Basic Auth + JWT artisanal par quelque chose de plus robuste et opérable, tout en conservant la compatibilité avec le mode simple actuel.

### Niveau 1 — Révocation des tokens & UI de gestion des API keys

- [x] ~~Clarifier et corriger la section JWT sur le site de documentation~~ — **obsolète depuis P18** (JWT supprimé des API keys, remplacé par tokens opaques)
- [x] Stocker les tokens émis en base (Redis ou autre) pour permettre la révocation explicite — aujourd'hui un token valide ne peut pas être invalidé avant expiration
- [x] Ajouter un endpoint `DELETE /tokens/:id` (Bearer admin) pour révoquer un token spécifique
- [x] Modifier le middleware auth pour vérifier la présence du token dans le store à chaque requête (blacklist ou whitelist selon le choix d'implémentation)
- [x] Créer une page d'administration (`/admin`) pour créer, lister et révoquer des API keys via une interface web

### Niveau 2 — Support OIDC / SSO (optionnel, activable via config)

Permettre de sécuriser la page `/admin` et l'émission de tokens via un fournisseur OIDC externe (Keycloak, Dex, Auth0, etc.) en remplacement ou en complément du Basic Auth actuel.

```yaml
auth:
  mode: basic        # "basic" (défaut, comportement actuel) | "oidc"
  oidc:
    issuer: https://keycloak.example.com/realms/gimme
    client_id: gimme
    client_secret: ""
    redirect_url: https://gimme.example.com/auth/callback
```

- [x] Définir l'interface `AuthProvider` (`Authenticate(ctx) (claims, error)`) pour abstraire Basic Auth et OIDC
- [x] Implémenter le provider OIDC (authorization code flow, `golang.org/x/oauth2` + `coreos/go-oidc`)
- [x] Protéger `/admin` avec le provider configuré
- [x] Documenter la configuration OIDC avec un exemple Keycloak
- [x] Mettre à jour le Helm chart : ajouter les paramètres `auth.mode`, `auth.oidc.*` dans `values.yaml` et le `ConfigMap`

## Priorité 17b — Findings code review PR#3 (MEDIUM)

Issues identifiées lors du code review automatisé du 2026-02-28 (`security/20260228_PR3-CODE-REVIEW.md`).

- [x] ~~`auth-manager.go:104-113` — `decodeToken` JWT algorithm check~~ — **obsolète depuis P18** (JWT supprimé des API keys)
- [x] `oidc-provider.go:163-177` — Nonce OIDC implémenté (cookie `gimme_oidc_nonce`)
- [x] `configs/config.go:134-136` — Validation longueur minimale secret (32 chars)
- [x] `application.go:151` — CORS configuré explicitement via `cors.allowed_origins`
- [x] `content-service.go:207` — Invalidation cache version partielle
- [x] `objectstorage-manager.go:146-156` — `ListObjects` logue et filtre les entrées S3 en erreur
- [x] `objectstorage-manager.go:159-176` — `ObjectExists` match exact (non préfixe)
- [x] `business-error.go:41-43` — `GetHTTPCode()` retourne `500` par défaut
- [x] `application.go:91-99` — Connexion Redis fermée proprement au shutdown
- [x] `Makefile:30-32` — `make test` propage l'exit code de `go test` via `exit $$TEST_EXIT`
- [x] `Makefile:3` — `gosec ./...` (3 points)
- [x] `Dockerfile` — architecture paramétrable via `GOARCH`

## Priorité 17c — Findings code review PR#3 (LOW)

- [x] `content-service.go:59` — `getVersion()` panic si un objet S3 ne contient pas `@` dans son nom — ajouter une vérification avant le split
- [x] `package-controller.go:64` — `file.Open()` : erreur silencée, un `nil` reader provoque un panic en aval — propager l'erreur avec un `500 Internal Server Error`
- [x] `internal/errors/business-error.go` — `GimmeError` n'implémente pas `Unwrap()` — `errors.Is()` ne traverse pas l'erreur wrappée, ajouter la méthode
- [x] Token store en mémoire ne purge jamais les tokens expirés — ajouter un ticker de nettoyage périodique (fuite mémoire lente sur le long terme)
- [x] `application.go:185-187` — Commentaire dit "5 seconds" mais le timeout est à 60s — corriger le commentaire
- [x] `package-controller.go:28-38` — `getSlice` ne valide pas les noms/versions vides — ajouter une validation et retourner un `400` explicite

## Priorité 18 — Tokens opaques & base de données relationnelle

Remplacer les JWT utilisés comme API keys par des tokens opaques stockés en base, aligné sur le modèle GitHub/GitLab. Nécessite l'introduction d'une base de données relationnelle (PostgreSQL recommandé).

### Niveau 1 — Modélisation des tokens dans Redis

Redis est déjà présent (cache P12) — pas de nouvelle dépendance. Les tokens sont stockés comme JSON sous la clé `token:<id>` avec TTL aligné sur `expires_at`.

- [x] Définir la structure de stockage Redis pour les tokens opaques (préfixe `token:`, TTL aligné sur `expires_at`)
- [x] S'assurer que Redis est obligatoire quand les tokens opaques sont activés (erreur au démarrage sinon)

### Niveau 2 — Migration des API keys vers tokens opaques

**Breaking change** : les tokens JWT existants ne sont pas migrés et deviennent invalides.
**Tous les tokens JWT émis avant cette version sont invalides et doivent être régénérés via `/admin`.**

- [x] Générer les tokens opaques côté serveur (`crypto/rand`, format `gim_<base62>`, ~40 chars)
- [x] Stocker uniquement le hash (`sha256`) en base — jamais le token en clair
- [x] Retourner le token en clair **une seule fois** à la création (comportement GitHub/GitLab)
- [x] Mettre à jour le middleware auth : `sha256(token reçu)` → lookup Redis → vérification `expires_at` et `revoked_at`
- [x] Mettre à jour `DELETE /tokens/:id` pour écrire `revoked_at` en Redis (révocation soft — le token reste visible mais invalide)
- [x] Mettre à jour la page `/admin` : tokens révoqués marqués visuellement, bouton Revoke remplacé par la date de révocation
- [x] JWT (`golang-jwt`) supprimé de la gestion des tokens API (conservé uniquement pour les sessions OIDC)
- [x] Mettre à jour les tests
- [x] Documenter le breaking change : indiquer que tous les tokens JWT émis avant cette version sont invalides et doivent être régénérés via `/admin`

### Niveau 3 — FileTokenStore : mode zéro-dépendance (fallback sans Redis)

Permettre de démarrer Gimme sans Redis en persistant les tokens dans un fichier JSON chiffré localement.
Le chiffrement utilise la `secret` existante (AES-GCM dérivé via HKDF) — zéro nouvelle configuration requise.
Si `cache.redis_url` est absent ou vide → FileTokenStore activé automatiquement avec un warning.

- [x] Implémenter `FileTokenStore` dans `internal/auth/file-token-store.go` (interface `TokenStore`)
- [x] Chiffrement AES-256-GCM du fichier JSON avec une clé dérivée de `config.Secret` (HKDF-SHA256)
- [x] Chemin configurable via `cache.file_path` (défaut : `./gimme-tokens.enc`)
- [x] Chargement au démarrage, flush synchrone à chaque mutation (Save/Revoke/Delete), purge périodique des tokens expirés
- [x] Mise à jour de `application.go` : sélection automatique FileTokenStore si Redis absent, RedisTokenStore sinon
- [x] Mise à jour de `configs/config.go` : `cache.redis_url` devient optionnel (validation conditionnelle)
- [x] Ajouter les tests unitaires pour `FileTokenStore`
- [x] Supprimer `MemoryTokenStore` (remplacé par `FileTokenStore`) et migrer tous les usages dans les tests
- [x] Documenter dans le README : mode standalone vs mode Redis
- [x] Introduire une config dédiée `tokenStore` pour découpler le choix du backend de stockage des tokens de la config cache Redis — remplacer l'inférence implicite `cache.redis_url != ""` par un champ explicite `tokenStore.mode: file|redis` (extensible à `postgres` etc.) — impacts : `configs/config.go` (nouvelle struct `TokenStoreConfig`, déplacement de `FilePath`), `internal/application/application.go` (switch sur `tokenStore.mode` au lieu de `cache.redis_url`), Helm chart (`values.yaml` + `configmap.yaml`, conditionner `tmp-volume` sur `tokenStore.mode == "file"`)
- [ ] Implémenter `PGTokenStore` dans `internal/auth/pg-token-store.go` (interface `TokenStore`) — backend PostgreSQL pour les déploiements qui disposent déjà d'une base relationnelle et préfèrent éviter Redis — nécessite l'introduction de `pgx` ou `database/sql` + driver `lib/pq`, une table `gimme_tokens` (id, hash, created_at, expires_at, revoked_at, metadata JSON), et la config `tokenStore.postgres.dsn` — dépend de la tâche précédente (`tokenStore.mode: postgres`)

## Priorité 19 — Finitions visuelles & contenu (non prioritaire)

- [x] Ajouter un vrai logo Gimme (fichier SVG/PNG) utilisé dans le site GitHub Pages (`docs/site/`) et les templates Go (`templates/`) — actuellement remplacé par un logotype texte + carré CSS
- [ ] Vidéo embarquée (à évaluer) : screencast montrant le déploiement + une utilisation concrète, intégré en section dédiée ou dans le Quickstart du site de documentation
- [ ] Tests E2E Playwright : couvrir la navigation dans un package, la copie d'URL, l'affichage des tailles, et les critères d'accessibilité de base (axe-core via `@axe-core/playwright`)

