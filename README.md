# Mail Service

Petit service Go qui reçoit les formulaires de contact de plusieurs sites et les envoie par e-mail via SMTP. Un seul service pour tous les projets : chaque site a son destinataire, ses origines autorisées, ses limites et ses champs propres, sans toucher au code.

Prévu pour tourner en Docker derrière Cloudflare (Tunnel ou proxy), qui fournit HTTPS et l'IP réelle du visiteur.

- [Fonctionnement](#fonctionnement)
- [Ajouter un projet](#ajouter-un-projet)
- [API](#api)
- [Côté client](#côté-client)
- [Configuration](#configuration)
- [Le mail reçu](#le-mail-reçu)
- [Sécurité et limites](#sécurité-et-limites)
- [Développement](#développement)
- [Déploiement](#déploiement)
- [Dépannage](#dépannage)

## Fonctionnement

```
Navigateur (formulaire)  ──POST JSON──►  https://mail.example.com/send/<site>
                                           │ CORS + origine du site
                                           │ limite par IP et par site
                                           │ pot de miel, validation
                                           ▼
                                     SMTP ──► destinataire(s) du site
```

- `POST /send` : le site **par défaut** (le portfolio), configuré par variables d'environnement. Comportement historique, inchangé.
- `POST /send/{site}` : les **autres projets**, déclarés dans `config/sites.json`.

L'envoi est synchrone : une réponse `200` veut dire que le serveur SMTP a accepté le message. Rien n'est mis en file d'attente, donc rien ne se perd silencieusement au redémarrage.

## Ajouter un projet

1. **Déclarer le site** dans `config/sites.json` sur le serveur (modèle : [`config/sites.example.json`](config/sites.example.json)), et mettre `SITES_FILE=/app/config/sites.json` dans `.env` la première fois :

   ```json
   {
     "sites": [
       {
         "id": "mon-site",
         "name": "Mon Site",
         "origins": ["https://www.example.com"],
         "to": ["contact@example.com"],
         "subject_prefix": "[Mon Site]",
         "lang": "fr",
         "require_contact": "phone",
         "daily_limit": 50
       }
     ]
   }
   ```

2. **Vérifier la config** sans toucher au service qui tourne :

   ```bash
   docker compose run --rm mail-service ./mail-service -check
   ```

3. **Recharger** : `docker compose up -d --force-recreate` (la config n'est lue qu'au démarrage).

4. **Côté site** : poster vers `https://<votre-domaine>/send/<id>` (voir [Côté client](#côté-client)) et ajouter l'origine du service à `connect-src` dans la CSP du site.

5. **Tester** depuis le site en production : le mail doit arriver chez le destinataire, avec l'objet préfixé.

`config/sites.json` n'est pas versionné (le dépôt est public et il contient des adresses) : il vit seulement sur le serveur. Le sauvegarder avec le `.env`.

## API

### `GET /health`

```json
{ "status": "ok", "timestamp": "2026-02-17T14:00:00Z" }
```

### `POST /send` et `POST /send/{site}`

En-tête `Content-Type: application/json` (`; charset=utf-8` accepté). Corps limité à 64 Ko.

```json
{
  "name": "Jean Dupont",
  "email": "jean@example.com",
  "phone": "06 12 34 56 78",
  "subject": "Demande de devis",
  "message": "Texte libre",
  "fields": { "Ville": "Anglet", "Type de travaux": "Peinture intérieure" },
  "company": ""
}
```

| Champ | Requis | Max | Notes |
|---|---|---|---|
| `name` | Oui | 100 | |
| `email` | Selon `require_contact` | 254 | Adresse simple (`a@b.fr`), sans nom affiché. Sert de `Reply-To`. |
| `phone` | Selon `require_contact` | 30 | Chiffres, espaces, `+ . - ( )`. |
| `subject` | Non | 200 | Préfixé par `subject_prefix` du site dans l'objet. |
| `message` | Oui, sauf si `fields` est rempli | 5000 | |
| `fields` | Non | 20 champs, libellé 50, valeur 1000 | Objet `"Libellé": "valeur"`. **L'ordre est conservé** dans le mail. Valeurs vides ignorées. |
| `company` | Non | | **Pot de miel** : doit rester vide. Rempli, le service répond `200` sans rien envoyer. |

| Code | Sens |
|---|---|
| `200` | Envoyé (ou pot de miel déclenché). `{"success": true, "message": "..."}` |
| `400` | JSON invalide |
| `403` | L'en-tête `Origin` n'est pas une origine de ce site |
| `404` | Site inconnu |
| `415` | Mauvais `Content-Type` |
| `422` | Validation : `{"success": false, "errors": [{"field": "phone", "message": "..."}]}` |
| `429` | Trop de requêtes (en-tête `Retry-After`) ou plafond journalier du site atteint |
| `502` | Le serveur SMTP a refusé ou n'a pas répondu à temps |

## Côté client

Exemple minimal, sans dépendance :

```html
<form id="contact">
  <input name="name" required>
  <input name="phone" type="tel">
  <input name="email" type="email">
  <input name="ville">
  <textarea name="message"></textarea>
  <!-- pot de miel : invisible pour les humains -->
  <input name="company" tabindex="-1" autocomplete="off" aria-hidden="true" style="position:absolute;left:-10000px">
  <button>Envoyer</button>
</form>
<script>
document.getElementById('contact').addEventListener('submit', async (e) => {
  e.preventDefault();
  const { ville, ...d } = Object.fromEntries(new FormData(e.target));
  const r = await fetch('https://mail.example.com/send/mon-site', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...d, fields: { Ville: ville } }),
  });
  const res = await r.json();
  // r.ok : afficher un remerciement. Sinon res.errors détaille les champs.
});
</script>
```

Bonnes pratiques :
- Toujours prévoir un repli en cas d'échec (téléphone et e-mail affichés) : le service peut être indisponible.
- Valider côté navigateur avec les mêmes règles, surtout le format du téléphone, pour éviter un `422` incompris.
- La CSP du site doit autoriser l'origine du service dans `connect-src`.

## Configuration

### Variables d'environnement (`.env`, secrets compris)

| Variable | Requis | Défaut | Rôle |
|---|---|---|---|
| `SMTP_HOST` | Oui | | Serveur SMTP |
| `SMTP_PORT` | Non | `587` | `587` (STARTTLS) ou `465` (TLS implicite) |
| `SMTP_USER` / `SMTP_PASSWORD` | Oui | | Identifiants SMTP |
| `SMTP_TIMEOUT_SECONDS` | Non | `10` | Durée max d'une session SMTP. Une seule nouvelle tentative sur erreur temporaire. |
| `MAIL_FROM` | Oui | | Expéditeur de tous les mails, sur un domaine dont vous gérez SPF/DKIM |
| `MAIL_TO` | Oui | | Destinataire(s) du site par défaut, séparés par des virgules |
| `CORS_ALLOWED_ORIGINS` | Oui | | Origines du site par défaut, séparées par des virgules |
| `SUBJECT_PREFIX` | Non | `[Portfolio Contact]` | Préfixe d'objet du site par défaut |
| `SITE_NAME` | Non | | Nom en pied de mail pour le site par défaut |
| `TIMEZONE` | Non | `Europe/Paris` | Fuseau de la date pour le site par défaut |
| `RATE_LIMIT_PER_MINUTE` | Non | `5` | Limite par IP et par minute, pour tout site qui n'en fixe pas |
| `SITES_FILE` | Non | | Fichier des sites : `/app/config/sites.json` en Docker |
| `SERVER_PORT` | Non | `8080` | |

### Fichier des sites (`config/sites.json`, sans secret)

| Clé | Requis | Défaut | Rôle |
|---|---|---|---|
| `id` | Oui | | Identifiant d'URL, `/send/<id>` : `a-z`, `0-9`, `-`, 32 caractères max. |
| `origins` | Oui | | Origines autorisées, au format `https://exemple.fr` (sans chemin ni `/` final). |
| `to` | Oui | | Liste des destinataires. |
| `name` | Non | | Nom en pied de mail (« Envoyé depuis … »). |
| `subject_prefix` | Non | `[<id>]` | Préfixe de l'objet. |
| `lang` | Non | `en` | `en` ou `fr` : libellés du mail. |
| `require_contact` | Non | `email` | `email`, `phone` ou `any` (l'un des deux). |
| `rate_limit_per_minute` | Non | `RATE_LIMIT_PER_MINUTE` | Limite par IP pour ce site. |
| `daily_limit` | Non | `0` (aucune) | Plafond de mails par 24 h pour tout le site : filet de sécurité contre un robot. |
| `timezone` | Non | `Europe/Paris` | Fuseau IANA de la date affichée. |

Une clé inconnue (faute de frappe) fait échouer le démarrage : mieux vaut une erreur claire qu'un réglage ignoré. `-check` affiche la config retenue.

## Le mail reçu

Un seul format, volontairement neutre pour tous les projets : texte noir sur fond blanc, polices du système, aucune couleur de marque. Il s'affiche proprement dans tous les clients et en mode sombre. Chaque mail a une partie texte et une partie HTML.

```
Objet : [Mon Site] Demande de devis

Demande de devis

Nom : Jean
Téléphone : 06 12 34 56 78
E-mail : non renseigné
Ville : Anglet
Type de travaux : Peinture

Texte du message…

--
Envoyé depuis Mon Site le 06/11/2026 à 09:30
```

- **Répondre** au mail écrit directement au visiteur (`Reply-To`), s'il a donné son e-mail.
- L'en-tête `X-Mail-Service-Site` indique le site d'origine : pratique pour un filtre de messagerie.
- Aperçu HTML : `make preview`, puis ouvrir `preview.html`.

## Sécurité et limites

| Protection | Détail |
|---|---|
| **Origine par site** | CORS autorise l'union des origines ; en plus, un POST navigateur vers `/send/<site>` depuis l'origine d'un autre site reçoit `403`. Un site ne peut pas écrire aux destinataires d'un autre. |
| **Limite par IP** | Par site et par minute (`rate_limit_per_minute`). L'IP vient de `CF-Connecting-IP` : fiable **uniquement** derrière Cloudflare. |
| **Plafond journalier** | `daily_limit` par site, compté sur les envois valides. |
| **Pot de miel** | Champ `company` : rempli, faux succès sans envoi. |
| **En-têtes du mail** | Retours à la ligne retirés du nom, du téléphone, de l'objet et des libellés ; e-mail sans nom affiché. Pas d'injection d'en-tête possible. |
| **SMTP borné** | Session limitée à `SMTP_TIMEOUT_SECONDS` ; une seule nouvelle tentative sur erreur temporaire (réseau, code 4xx), jamais après acceptation du message. |
| **Journaux** | Site, statut et durée seulement : ni nom, ni e-mail, ni message dans les logs. |
| **En-têtes HTTP** | HSTS, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store` sur toutes les réponses. |
| **Conteneur** | Utilisateur non-root, système de fichiers en lecture seule, config montée en lecture seule, port lié à `127.0.0.1`, healthcheck. |

Les compteurs de limites sont en mémoire : ils repartent de zéro au redémarrage. C'est suffisant pour une seule instance ; avec plusieurs instances, il faudrait un stockage partagé (Redis, par exemple).

## Développement

```bash
cp .env.example .env     # puis renseigner le SMTP
make run                 # go run ./cmd/server
make test                # go vet + go test -race
make check               # valide .env et SITES_FILE puis quitte
make preview             # écrit preview.html, un exemple de mail
```

Les tests couvrent la config, la validation, le routage par site, les limites, l'encodage du mail (quoted-printable, accents, longueur des lignes) et l'envoi sur un faux serveur SMTP (nouvelle tentative, délai dépassé).

## Déploiement

`docker compose up -d --build` sur le serveur. Le workflow `.github/workflows/deploy.yml` peut déclencher ce déploiement à chaque merge sur `main` : l'URL et le secret du webhook sont des secrets du dépôt (`WEBHOOK_URL`, `WEBHOOK_SECRET`), jamais écrits dans le code.

Le `.env` et `config/sites.json` ne sont pas dans le dépôt : ils se modifient sur le serveur, suivis de `docker compose up -d --force-recreate`.

## Dépannage

| Symptôme | Piste |
|---|---|
| Le service ne démarre pas | `docker compose logs mail-service` : la ligne `config:` nomme la variable ou la clé fautive. `-check` aussi. |
| `403 Origin not allowed` | L'origine manque dans `origins`, a un `/` final, ou `www` diffère. |
| `404 Unknown site` | `id` absent de `sites.json`, `SITES_FILE` non défini, ou conteneur pas recréé. |
| `422` | Lire `errors` dans la réponse : souvent le téléphone ou l'e-mail. |
| `429` | Limite par IP ou plafond journalier : voir les logs (`daily limit reached`). |
| `502 Failed to send message` | Problème SMTP : chercher `failed to send email` dans les logs. Identifiants SMTP expirés ? |
| Bloqué par la CSP du site | Ajouter l'origine du service à `connect-src`. |
| Mail en spam | Vérifier SPF, DKIM et DMARC du domaine de `MAIL_FROM` ; marquer « pas un spam » une première fois. |
