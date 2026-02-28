# Code Review — PR #3: Project Refresh

**Date**: 2026-02-28
**Branch**: `refactor/project-refresh` → `main`
**Scope**: 102 files changed, +11 165 / -1 748 lines, 40 commits
**Reviewer**: Claude Opus 4.6 (automated)

---

## Verdict global

Le travail est massif et globalement de bonne qualité : modernisation Go 1.26, corrections de bugs critiques, ajout de cache, métriques, OIDC, Helm chart, et refonte UI. Le code suit les conventions du projet et les tests sont significativement améliorés. Cependant, quelques findings méritent attention avant merge.

---

## Findings CRITIQUES (à corriger)

### 1. `POST /create-token` non protégé en mode OIDC — HIGH

**`api/auth-controller.go:73` + `configs/config.go:126-133`**

En mode `auth.mode=oidc`, la config skip la validation de `AdminUser`/`AdminPassword`. Or `POST /create-token` utilise `gin.BasicAuth` avec ces valeurs potentiellement vides. Un attaquant peut s'authentifier avec `Authorization: Basic Og==` (base64 de `:`) et minter des tokens API arbitraires.

### 2. Data race dans `RemoveObjects` — HIGH

**`internal/storage/objectstorage-manager.go:206-243`**

`removeErrors` est écrit par la goroutine de listing ET la goroutine principale simultanément — c'est un data race classique. Nécessite un mutex ou un channel.

### 3. `docker login -p` expose le password dans `/proc` — HIGH

**`.github/workflows/build.yml:110` + `release.yml:37`**

Utiliser `echo "$PASS" | docker login --password-stdin` à la place.

### 4. Shell injection via `github.ref_name` — HIGH

**`.github/workflows/build.yml:101` + `release.yml:39,58`**

`${{ github.ref_name }}` interpolé directement dans du shell. Un nom de branche malicieux peut exécuter du code. Assigner à une variable d'environnement d'abord.

---

## Findings MEDIUM (à planifier)

| # | Fichier | Issue |
|---|---------|-------|
| 5 | `auth-manager.go:104-113` | Pas de vérification d'algorithme dans `decodeToken` (algorithm confusion risk) — le OIDC provider le fait bien, pas le auth manager |
| 6 | `oidc-provider.go:163-177` | Pas de paramètre `nonce` OIDC (replay d'ID token) |
| 7 | `configs/config.go:134-136` | Pas de longueur minimale pour le secret JWT (1 char accepté) |
| 8 | `application.go:151` | `cors.Default()` autorise toutes les origines |
| 9 | `content-service.go:207` | Invalidation cache incomplète : supprimer `1.0.3` n'invalide pas le cache de `pkg@1.0` qui résolvait vers `1.0.3` |
| 10 | `objectstorage-manager.go:146-156` | `ListObjects` ignore les erreurs S3 (entries avec `Err != nil` incluses silencieusement) |
| 11 | `objectstorage-manager.go:159-176` | `ObjectExists` match par préfixe — `1.0.0` matche aussi `1.0.0-beta` |
| 12 | `business-error.go:41-43` | `GetHTTPCode()` retourne `0` pour un `Kind` inconnu (interprété comme 200 par Go) |
| 13 | `application.go:91-99` | Connexion Redis jamais fermée au shutdown |
| 14 | `Makefile:30-32` | `make test` retourne toujours succès car l'exit code de `go test` est perdu (`;` au lieu de `&&`) |
| 15 | `Makefile:3` | `gosec ./..` au lieu de `gosec ./...` (2 dots au lieu de 3) |
| 16 | `Dockerfile` | `make release` hardcode `GOARCH=amd64` — incompatible avec les builds multi-arch |

---

## Findings LOW (nice to have)

- Token store en mémoire ne purge jamais les tokens expirés (fuite mémoire lente)
- `getVersion()` panic si un objet S3 ne contient pas `@` (`content-service.go:59`)
- `extractToken` n'exige pas le scheme `Bearer` (`auth-manager.go:95-101`)
- `List()` admin expose les tokens bruts — le principe "affichage unique à la création" est violé
- Pas de `Unwrap()` sur `GimmeError` — `errors.Is()` ne traverse pas l'erreur wrappée
- Comment dit "5 seconds" mais timeout = 60s (`application.go:185-187`)
- `file.Open()` error silencée dans `package-controller.go:64` — nil reader = panic
- Health controller sans tests unitaires (`/healthz`, `/readyz`)
- Pas de test pour le happy path complet du callback OIDC
- Grafana en anonymous admin dans le docker-compose dev
- Pas de `jti` dans le cookie session OIDC (pas de révocation possible)
- State cookie empty-string edge case dans `oidc-provider.go:182-189`
- `ContentService` utilisé comme type concret, pas interface — pas mockable dans les tests controllers
- `getSlice` ne valide pas les noms/versions vides (`package-controller.go:28-38`)
- Pas de `startupProbe` Kubernetes dans le Helm chart

---

## Points positifs

- Excellente séparation unit / integration tests
- Bonne utilisation de `errgroup` pour le parallélisme d'upload
- OIDC bien implémenté (state cookie, domain-separated signing, SameSite=Lax)
- Helm chart solide (securityContext, readOnlyRootFilesystem, capabilities dropped)
- Cache-Control headers bien pensés (immutable pour pinned, 300s pour partial)
- Métriques Prometheus bien structurées avec 6 métriques applicatives
- Docker hardened (non-root, multi-stage, CGO disabled)
- Init-garage idempotent et bien documenté
- Bonne couverture de tests pour les nouveaux composants (OIDC, cache, metrics, admin)
- Gestion propre des erreurs avec `GimmeError` typé
