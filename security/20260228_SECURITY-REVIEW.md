# Rapport de revue de sécurité — `refactor/project-refresh` (P13–P17)

**Date** : 2026-02-28
**Branche** : `refactor/project-refresh`
**Périmètre** : Commits depuis la revue précédente (2026-02-25) — P13 métriques, P14 docs, P15 templates, P16 Helm, P17 OIDC/tokens
**Outil** : Claude Code — analyse statique + agents de validation croisée (7 findings initiaux, 4 validés, 0 confirmé)

---

## Résultat global

**Aucune vulnérabilité exploitable n'a été identifiée.** Sept pistes candidates ont été investiguées par analyse statique ; les quatre retenues après le filtre de confiance initiale (≥ 7/10) ont toutes été invalidées par des agents de validation indépendants.

---

## Périmètre analysé

Fichiers principaux examinés :

| Fichier | Surface exposée |
|---------|----------------|
| `internal/auth/oidc-provider.go` | Flux OIDC/SSO, cookies de session |
| `internal/auth/auth-manager.go` | Validation JWT, middleware |
| `internal/auth/memory-token-store.go` | Stockage et révocation des tokens |
| `api/admin-controller.go` | Endpoints admin (création/révocation tokens) |
| `templates/admin.tmpl` | UI admin (formulaire, liste tokens) |
| `internal/application/application.go` | Bootstrap, routeur, wiring |
| `configs/config.go` | Validation configuration OIDC |
| `api/auth-controller.go` | `POST /create-token` |
| `api/package-controller.go` | `POST /packages`, `DELETE /packages/:package` |
| `internal/content/content-service.go` | Extraction ZIP, upload S3 |
| `internal/cache/redis-cache.go` | Client Redis/Valkey |

---

## Findings candidats (tous écartés)

### 1. Algorithm Confusion JWT — `internal/auth/auth-manager.go:105`

**Verdict** : Faux positif (confiance résiduelle : 2/10)

**Analyse** : La fonction `decodeToken()` n'effectue pas de vérification explicite de l'algorithme de signature dans le callback de `jwt.Parse()`. En apparence, un attaquant pourrait soumettre un token signé avec `"alg": "none"`.

**Pourquoi ce n'est pas exploitable** :
- `golang-jwt/jwt` v4.5.2 rejette explicitement l'algorithme `"none"` au niveau du parseur, avant même d'invoquer le callback — aucun `SigningMethodNone` n'est enregistré dans le registre par défaut.
- La `TokenStore` effectue une vérification de liste blanche sur chaque requête : même si un token parvenait à passer le parsing JWT (impossible), il ne figurerait pas dans la whitelist.
- Le pattern de validation explicite (`if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok`) est correctement appliqué dans `oidc-provider.go:101` — son absence dans `auth-manager.go` est un défaut de cohérence, pas une vulnérabilité.

**Recommandation qualité (non bloquante)** : Harmoniser les deux fichiers en ajoutant la vérification explicite dans `decodeToken()` pour la lisibilité.

---

### 2. XSS via nom de token dans le template admin — `templates/admin.tmpl:438,453`

**Verdict** : Faux positif (confiance résiduelle : 9/10 d'être faux positif)

**Analyse** : Le template insère `{{ .Name }}` et `{{ .ID }}` dans des attributs HTML (`data-id`, `data-name`, `aria-label`), ce qui semble exposé à l'injection HTML.

**Pourquoi ce n'est pas exploitable** :
- Gin utilise `html/template` (importé dans `api/templates.go:4`), qui applique un échappement contextuel automatique. Dans un contexte d'attribut HTML, les guillemets et chevrons sont encodés (`"` → `&#34;`, `<` → `&lt;`).
- Aucun usage de `template.HTML`, `safeHtml`, ou d'autres mécanismes désactivant l'échappement n'est présent dans les templates.
- Le JavaScript injecté côté client via `innerHTML` utilise en plus une fonction `escHtml()` (lignes ~684-690), ajoutant une défense en profondeur.

---

### 3. Path Traversal ZIP — `internal/content/content-service.go:107`

**Verdict** : Écarté au filtre initial (confiance : 7/10, seuil requis : 8/10)

**Analyse** : La regex `^[a-zA-Z0-9-_]+` remplace uniquement le préfixe alphanumérique d'un chemin ZIP, laissant potentiellement des séquences `../`. Cependant, S3/Garage traite les clés d'objets comme des chaînes opaques — aucun mécanisme de résolution de chemin n'est appliqué. Ce point était déjà documenté dans la revue précédente (2026-02-25).

---

### 4. Token retourné dans la réponse JSON — `api/auth-controller.go:64`

**Verdict** : Faux positif (confiance résiduelle : 9/10 d'être faux positif)

**Analyse** : `POST /create-token` et `POST /tokens` retournent le JWT en clair dans la réponse JSON.

**Pourquoi ce n'est pas exploitable** :
- Retourner le token à la création est architecturalement obligatoire : le token n'est jamais stocké en clair côté serveur après création, il n'existe que dans cette réponse. Supprimer ce comportement rendrait le token inutilisable.
- Gin en mode `Default()` ne logue pas les corps de réponse — il n'existe aucun middleware de logging de réponse dans le code.
- C'est le pattern standard de toutes les grandes APIs (GitHub, Stripe, AWS).

---

### 5. Cookie de session OIDC sans TLS forcé — `internal/auth/oidc-provider.go:153`

**Verdict** : Écarté au filtre initial (confiance : 7/10, seuil requis : 8/10)

**Analyse** : Le flag `secureCookies` est configurable. Si positionné à `false` (usage local), le cookie de session OIDC circule en clair. Risque réel uniquement en cas de mauvaise configuration opérationnelle, pas de faille applicative.

---

### 6. Redirect URL OIDC non validée — `internal/auth/oidc-provider.go:73`

**Verdict** : Faux positif (confiance résiduelle : 9/10 d'être faux positif)

**Analyse** : La valeur de `RedirectURL` n'est pas validée (format, domaine) dans le code applicatif.

**Pourquoi ce n'est pas exploitable** :
- La valeur provient exclusivement de la configuration (fichier `gimme.yml` ou variable d'environnement `GIMME_AUTH_OIDC_REDIRECT_URL`) — c'est une **entrée de confiance** contrôlée par l'opérateur, pas par un utilisateur.
- Modifier une variable d'environnement d'un processus en production nécessite un accès système complet (compromission préalable).
- Le fournisseur OIDC (Keycloak, Auth0…) valide la `redirect_uri` contre sa liste blanche enregistrée — protection en couche indépendante de l'application.

---

### 7. Validation du nom de token — `api/auth-controller.go:28`

**Verdict** : Écarté au filtre initial (confiance : 7/10, seuil requis : 8/10)

**Analyse** : Seule la longueur du nom est validée (max 255 caractères), pas le jeu de caractères. Pas d'impact de sécurité démontrable : le nom est stocké et réaffiché via `html/template` avec échappement automatique.

---

## Points positifs identifiés (nouveaux sur P13–P17)

| # | Description | Fichier(s) |
|---|-------------|------------|
| ✅ | **TokenStore avec liste blanche** — chaque requête authentifiée vérifie que le token JWT figure dans la whitelist en mémoire ; les tokens révoqués sont rejetés immédiatement, même si le JWT est encore valide cryptographiquement | `internal/auth/memory-token-store.go`, `internal/auth/auth-manager.go` |
| ✅ | **Secret de session OIDC dérivé** — le secret de signature des cookies de session est dérivé via un préfixe (`"oidc-session:"` + secret global) pour empêcher la réutilisation des tokens API comme cookies de session | `internal/auth/oidc-provider.go:59` |
| ✅ | **Protection CSRF sur le flux OIDC** — un cookie d'état (`gimme_oidc_state`) aléatoire est émis avant la redirection vers l'IdP et validé à la réception du callback, avec nettoyage sur échec | `internal/auth/oidc-provider.go:131-148` |
| ✅ | **Cookie de session HttpOnly + SameSite=Lax** — le cookie de session OIDC est protégé contre la lecture JavaScript (`httpOnly=true`) et les requêtes cross-site (`SameSite=Lax`) | `internal/auth/oidc-provider.go:220` |
| ✅ | **Validation OIDC explicite de l'algorithme** — `oidc-provider.go` vérifie explicitement `token.Method.(*jwt.SigningMethodHMAC)` avant de retourner la clé, contrairement au JWT API qui délègue à la bibliothèque | `internal/auth/oidc-provider.go:101` |
| ✅ | **Avertissement sur secret OIDC vide** — un warning logrus est émis si `client_secret` est vide, sans pour autant bloquer le démarrage (cas PKCE) | `internal/auth/oidc-provider.go:47` |
| ✅ | **Refus d'accès JSON-aware** — le middleware `rejectUnauthenticated` retourne `401 JSON` quand `Accept: application/json`, évitant une redirection HTML vers l'IdP pour les clients API | `internal/auth/oidc-provider.go` |
| ✅ | **UUID non devinable pour les IDs de token** — les identifiants de token utilisent `github.com/google/uuid` v4 (128 bits d'entropie) | `internal/auth/memory-token-store.go` |
| ✅ | **Validation de configuration OIDC au démarrage** — les paramètres requis (`issuer`, `client_id`, `redirect_url`) sont vérifiés au boot via `configs/config.go`, empêchant un démarrage silencieusement mal configuré | `configs/config.go:160-168` |

---

## Conclusion

La branche `refactor/project-refresh` dans son état actuel (incluant P13 à P17) présente une **posture de sécurité solide**. Le nouveau système d'authentification (OIDC, token management, admin UI) a été implémenté avec les protections attendues : CSRF, HttpOnly, SameSite, dérivation de secret, liste blanche, UUIDs.

Aucune action corrective de sécurité n'est requise avant fusion.

**Recommandation qualité optionnelle** (non bloquante) : harmoniser `auth-manager.go:decodeToken()` avec le pattern de vérification d'algorithme déjà présent dans `oidc-provider.go:101` pour la cohérence du code.
