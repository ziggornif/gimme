# Code Review v2 — PR #3: Project Refresh

**Date**: 2026-02-28
**Branch**: `refactor/project-refresh` → `main`
**Scope**: 113 files changed, +12 296 / -1 938 lines
**Reviewer**: Claude Opus 4.6 (automated)
**Contexte**: Seconde revue complète, vérification des correctifs appliqués suite à la revue initiale

---

## Verdict global

**Excellent travail de correction.** Sur les 27 findings identifiés dans la revue initiale, **22 ont été corrigés** (4 HIGH, 12 MEDIUM, 6 LOW). Les 5 restants sont tous de sévérité LOW et acceptables pour un merge. 6 nouveaux findings ont été identifiés (1 MEDIUM, 5 LOW).

---

## Tableau de synthèse

| ID  | Sévérité | Description | Statut |
|-----|----------|-------------|--------|
| H1  | HIGH     | `POST /create-token` non protégé en mode OIDC | **CORRIGE** |
| H2  | HIGH     | Data race dans `RemoveObjects` | **CORRIGE** |
| H3  | HIGH     | `docker login -p` expose le mot de passe | **CORRIGE** |
| H4  | HIGH     | Shell injection via `github.ref_name` | **CORRIGE** |
| M5  | MEDIUM   | Pas de vérification d'algorithme dans `decodeToken` | **CORRIGE** (architecture changée — tokens opaques) |
| M6  | MEDIUM   | Pas de paramètre `nonce` OIDC | **CORRIGE** |
| M7  | MEDIUM   | Pas de longueur minimale pour le secret JWT | **CORRIGE** (min 32 chars) |
| M8  | MEDIUM   | `cors.Default()` autorise toutes les origines | **CORRIGE** (configurable, défaut documenté) |
| M9  | MEDIUM   | Invalidation cache incomplète pour versions partielles | **CORRIGE** |
| M10 | MEDIUM   | `ListObjects` ignore les erreurs S3 | **CORRIGE** |
| M11 | MEDIUM   | `ObjectExists` match par préfixe (`1.0.0` matche `1.0.0-beta`) | **CORRIGE** (trailing `/`) |
| M12 | MEDIUM   | `GetHTTPCode()` retourne `0` pour `Kind` inconnu | **CORRIGE** (défaut 500) |
| M13 | MEDIUM   | Connexion Redis jamais fermée au shutdown | **CORRIGE** |
| M14 | MEDIUM   | `make test` retourne toujours succès | **CORRIGE** |
| M15 | MEDIUM   | `gosec ./..` au lieu de `./...` | **CORRIGE** |
| M16 | MEDIUM   | Dockerfile hardcode `GOARCH=amd64` | **CORRIGE** (build args) |
| L1  | LOW      | Token store ne purge jamais les tokens expirés | **CORRIGE** (purgeLoop 5min / TTL Redis) |
| L2  | LOW      | `getVersion()` panic si pas de `@` | **CORRIGE** |
| L3  | LOW      | `extractToken` n'exige pas le scheme `Bearer` | **CORRIGE** |
| L4  | LOW      | `List()` admin expose les tokens bruts | **CORRIGE** (stockage SHA-256 uniquement) |
| L5  | LOW      | Pas de `Unwrap()` sur `GimmeError` | **CORRIGE** |
| L6  | LOW      | Commentaire/timeout incohérent | **CORRIGE** |
| L7  | LOW      | `file.Open()` erreur silencée dans `package-controller.go` | **CORRIGE** |
| L8  | LOW      | Health controller sans tests unitaires | **CORRIGE** |
| L9  | LOW      | Pas de test pour le happy path du callback OIDC | OUVERT |
| L10 | LOW      | Grafana en anonymous admin dans docker-compose dev | OUVERT |
| L11 | LOW      | Pas de `jti` dans le cookie session OIDC | OUVERT |
| L13 | LOW      | `ContentService` utilisé comme type concret, pas interface | OUVERT |
| L15 | LOW      | Pas de `startupProbe` dans le Helm chart | OUVERT |

---

## Nouveaux findings

### N1 (MEDIUM) — `createPackage` ne valide pas les champs `name` et `version`

**`api/package-controller.go:66-68`**

Les valeurs `name` et `version` du formulaire POST sont passées directement à `contentService.CreatePackage()` sans validation. Un `name` ou `version` vide créerait des objets S3 avec des clés malformées (`@/filename` ou `name@/filename`). La fonction `getSlice` valide correctement sur les chemins de lecture, mais le chemin d'écriture (`createPackage`) n'a pas de validation équivalente.

**Recommandation** : Ajouter la même validation que `getSlice` (nom et version non vides) en début de `createPackage`.

---

### N2 (LOW) — Le hint admin dit "15 min" mais le défaut réel est 90 jours

**`templates/admin.tmpl:410` vs `internal/auth/auth-manager.go:83-86`**

```html
<span id="expiry-hint" class="form-hint">Leave blank -- defaults to 15 min.</span>
```

Mais le code applique un défaut de 90 jours :
```go
expiresAt = time.Now().Add(90 * 24 * time.Hour)
```

Le hint est obsolète et trompeur. Risque fonctionnel, pas de sécurité.

---

### N3 (LOW) — Pas de `ReadHeaderTimeout` sur le serveur HTTP

**`internal/application/application.go:199-202`**

```go
server := &http.Server{
    Addr:    fmt.Sprintf(":%s", app.config.AppPort),
    Handler: router,
}
```

Le serveur HTTP ne définit pas `ReadHeaderTimeout`. Cela expose potentiellement à des attaques de type slowloris (connexions maintenues ouvertes indéfiniment par envoi lent des headers).

**Recommandation** : Ajouter `ReadHeaderTimeout: 10 * time.Second`.

---

### N4 (LOW) — `release.yml` ne sanitize pas `ref_name` pour le tag Docker

**`.github/workflows/release.yml:39-45`**

Contrairement à `build.yml` qui sanitize le nom de branche avec `sed`, `release.yml` utilise `REF_NAME` directement comme tag Docker. Risque faible car déclenché uniquement sur des événements `release`, mais manque de cohérence avec `build.yml`.

---

### N5 (LOW) — Erreur `FormFile` ignorée dans `createPackage`

**`api/package-controller.go:66`**

```go
file, _ := c.FormFile("file")
```

L'erreur de `FormFile` est ignorée. `ValidateFile` vérifie `nil` sur la ligne suivante, mais un message d'erreur plus informatif pourrait être fourni.

---

### N6 (LOW) — `sed` injection potentielle dans `release.yml`

**`.github/workflows/release.yml:63-69`**

La commande `sed` utilise une substitution en double-quotes avec `CHART_VERSION`. Si la version contenait des métacaractères sed (`/`, `&`, `\`), la commande pourrait échouer ou produire un résultat inattendu. Risque pratiquement nul car les tags de release sont créés manuellement.

---

## Findings LOW restants de la v1 (acceptables pour merge)

### L9 — Pas de test pour le callback OIDC

Le flux OIDC callback est intrinsèquement difficile à tester unitairement (nécessite mock du provider OAuth2 et de l'échange de tokens). Acceptable en l'état, à couvrir via tests d'intégration ou e2e ultérieurement.

### L10 — Grafana anonymous admin dans docker-compose

Les fichiers `docker-compose.yml` d'exemple configurent Grafana avec `GF_AUTH_ANONYMOUS_ORG_ROLE=Admin`. C'est un fichier de développement, mais ajouter un commentaire d'avertissement ou passer à `Viewer` serait une amélioration.

### L11 — Pas de `jti` dans le cookie session OIDC

Les cookies de session OIDC n'ont pas de claim `jti`, ce qui empêche la révocation individuelle côté serveur. Acceptable car les sessions ont un TTL court (8h).

### L13 — ContentService comme type concret

`ContentService` est un struct concret, ce qui rend le mocking difficile dans les tests des controllers. Amélioration de testabilité, pas de risque fonctionnel.

### L15 — Pas de startupProbe dans le Helm chart

Le Helm chart définit `livenessProbe` et `readinessProbe` mais pas de `startupProbe`. Le `initialDelaySeconds: 5` sur la liveness probe atténue partiellement le risque.

---

## Points positifs (confirmés et nouveaux)

- **Tous les findings HIGH corrigés** — data race, auth bypass, CI secrets, shell injection
- **Tokens opaques SHA-256** — l'architecture JWT pour les tokens API a été remplacée par des tokens opaques hashés, éliminant toute une classe de vulnérabilités (algorithm confusion, secret faible, etc.)
- **Purge automatique** des tokens expirés (memory store + Redis TTL)
- **Nonce OIDC** correctement implémenté avec validation
- **Shutdown graceful** avec fermeture des connexions Redis et cache
- **Validation d'entrées** renforcée (`getSlice`, `getVersion`, `extractToken`)
- **Helm chart sécurisé** — securityContext, readOnlyRootFilesystem, capabilities dropped
- **Tests étoffés** — health controller, token store, cache, métriques

---

## Conclusion

La PR est dans un **excellent état** pour le merge. Les 4 findings HIGH et 12 findings MEDIUM de la revue initiale ont tous été corrigés. Les issues restantes sont toutes LOW et non bloquantes.

**Seul finding actionnable avant merge** : N1 (MEDIUM) — ajouter la validation des champs `name`/`version` dans `createPackage` pour être cohérent avec le chemin de lecture.

**Score de confiance** : 9/10 pour le merge après correction de N1.
