# ADR-005 — Corrélation Postfix par preuves

Statut : accepté le 3 octobre 2026.

## Contexte
Queue IDs réutilisables, multiples destinataires, retries, rejets NOQUEUE, réinjections et hôtes distincts rendent une clé unique simple incorrecte.
## Options
Fusion par Queue ID ou Message-ID ; graphe de générations de file et liens typés.
## Décision
Événements immuables, QueueInstance `(instance, queue_id, génération)`, graphe révisable ; lien confirmé uniquement sur preuve corroborée.
## Raisons
Préserve le détail par destinataire et distingue les faits observés des candidats.
## Conséquences
Tentatives multiples, états par destinataire, résumé mixed/incomplete, NOQUEUE sans file, recalcul sur import tardif.
## Limites
Un parcours sans journaux des relais peut rester inconnu ; `sent` SMTP ne prouve pas la remise en boîte.
