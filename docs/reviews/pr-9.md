# Revue de la PR #9 — fondations des parseurs

Revue du 4 octobre 2026. Référence initiale :
`932bef1e20c3573b243d1ddbdfd6ec42e4fdb4d6`, PR
[QueueAtlas : conception validée et fondations des parseurs Postfix](https://github.com/Coubiac/QueueAtlas/pull/9).

## Périmètre

Enveloppes syslog RFC3164/RFC5424, observations typées, extraction Postfix,
corpus synthétique et CI. Revue du coordinateur et audit par un agent indépendant
en lecture seule, selon le workflow du cadrage initial. SQLite, FileSource,
corrélation, API, interface et packaging sont hors de cette PR.

Entrée bornée à 64 Kio avant extraction, 32 champs au plus ; aucun verdict de
remise finale ni corrélation entre lignes. Les années/fuseaux proviennent du contexte
fourni et la qualité du temps est conservée. Données déclarées par les journaux
non assimilées à une identité d'instance de confiance. La CI a des permissions
contents:read, des actions épinglées et ne conserve pas les credentials checkout.

## Problème trouvé et corrigé

P2 : l'ancienne regex des champs séparés par espaces fermait une adresse au premier
`>`, même situé dans un local-part entre guillemets. Exemple synthétique :

```text
Oct  3 12:34:56 mx postfix/pickup[1]: ABC123: uid=1001 from=<"x> to=<victim@example.org>"@example.org>
```

Elle produisait un faux champ `to` depuis le texte de l'expéditeur. Le logging
pickup utilise `from=<%s>` et le format d'adresse externe peut conserver ces
caractères entre guillemets : [pickup.c](https://github.com/vdukhovni/postfix/blob/master/postfix/src/pickup/pickup.c)
et [quote_822_local.c](https://github.com/vdukhovni/postfix/blob/master/postfix/src/global/quote_822_local.c).

Le scanner sépare désormais uniquement hors guillemets et hors adresse entre
chevrons ; il respecte les quotes échappées et n'extrait pas de clés internes
d'une valeur incomplète. Expéditeur conservé intégralement, aucun faux destinataire,
status ou uid. Régressions couvrant les caractères hostiles, l'échappement, sender
vide, métadonnées smtpd et valeur incomplète ; seed fuzz ajouté. Scanner linéaire
dans la borne d'entrée, sans dépendance ajoutée. Relecture indépendante du correctif
et tests ciblés réussis, aucun autre blocage concret identifié dans ce périmètre.

## Vérifications réalisées

- `go test ./...` et `go vet ./...` sur le checkout exact de M1, puis le correctif.
- Builds Linux amd64 et arm64 sans CGO.
- Fuzz Postfix et syslog, 3 secondes chacun avec deux workers : réussis.
- Manifest : 30 scénarios, tous les fichiers référencés présents ; contenu gzip
  identique au fichier normal correspondant.
- `git diff --check` réussi.

La CI de la tête publiée doit réussir avant fusion. Son résultat et le commit
de fusion sont consignés dans la PR et dans le point de reprise du chantier.

## Limites de la conclusion

Audit de code et données synthétiques, sans instance Postfix réelle ; fuzzing court,
sans garantie d'absence de toutes les erreurs. Le corpus énonce les futurs résultats
de corrélation, mais les tests M1 ne valident pas un moteur de corrélation. L'interface
devra échapper les données affichées et les futures sources fournir l'identité fiable.
M1 livre des fondations de parseurs ; aucun service ou paquet installable n'est livré.
