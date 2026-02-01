````md
# 🐛 ASWORM — Activity & Status Work Output Recording Monitor (Windows)

ASWORM est un utilitaire léger **Windows-only** développé en **Go**.  
Il permet de surveiller l’activité d’un poste de travail en mesurant :

- le **temps d’inactivité (idle time)** ⏳  
- les **mouvements de la souris** 🖱️  

Puis il génère des fichiers journaliers de logs 📄 indiquant le mode d’activité actuel.

> ✅ ASWORM ne capture **pas** les frappes clavier ⌨️, ni l’écran 🖥️, ni les applications utilisées 📌.  
> Il se limite uniquement aux signaux d’activité (idle + souris).

---

## ✨ Fonctionnalités

- 🪟 Compatible uniquement Windows (`//go:build windows`)
- ⚙️ Basé sur les API Win32 via `golang.org/x/sys/windows`
- 🔍 Détection :
  - Temps d’inactivité (`GetLastInputInfo`) ⏳
  - Mouvement souris (`GetCursorPos`) 🖱️
- 📊 Fenêtre glissante d’activité (par défaut : 30 min)
- 🚦 Classification automatique en 3 modes :
  - 💪 `HIGH_PRODUCTIVE`
  - 🙂 `SIMPLE_PRODUCTIVE`
  - 😴 `IDLE`
- 📅 Rotation automatique des logs par jour
- 💾 Synchronisation régulière sur disque
- 🛑 Arrêt propre via `Ctrl+C`

---

## 🧠 Comment ça fonctionne ?

ASWORM effectue un échantillonnage toutes les secondes (configurable).

---

### ⏳ Mesure du temps d’inactivité

Le programme utilise :

- `GetLastInputInfo` → dernière interaction clavier/souris  
- `GetTickCount64` → uptime système  

Le calcul est sécurisé contre le wrap-around du compteur Windows 🔄.

---

### ✅ Définition d’un échantillon actif

Un échantillon est considéré comme actif si :

```text
idleNow < ActiveIfIdleLessThan
````

Valeur par défaut :

* ⏱️ 30 secondes

---

### 📊 Score d’activité (Rolling Window)

ASWORM conserve les échantillons sur :

* 🕐 30 minutes (`WindowSize`)

Puis calcule :

```text
activeRatio = activeSamples / totalSamples
```

---

## 🚦 Modes d’activité

Les modes sont déterminés selon ces seuils :

| Mode 🏷️              | Condition ✅           |
| -------------------- | --------------------- |
| 😴 IDLE              | Idle continu ≥ 30 min |
| 💪 HIGH_PRODUCTIVE   | activeRatio ≥ 60%     |
| 🙂 SIMPLE_PRODUCTIVE | activeRatio ≥ 30%     |
| 😴 IDLE              | Sinon                 |

---

## 📄 Système de Logs

Les logs sont enregistrés dans :

```text
C:\ProgramData\ActivityMonitor\
```

Format journalier :

```text
activity-YYYY-MM-DD.log
```

---

### 📝 Exemple de logs

🚀 Démarrage :

```text
[2026-02-01T09:00:00Z] START (logs in C:\ProgramData\ActivityMonitor as activity-YYYY-MM-DD.log)
```

🖱️ Mouvement souris :

```text
[time] MOUSE MOVE: (412,305) delta=(+5,-2)
```

🔄 Changement de mode :

```text
[time] MODE CHANGE: HIGH_PRODUCTIVE idleNow=3s activeRatio=65% samples=1800
```

📌 Statut périodique :

```text
[time] STATUS: mode=SIMPLE_PRODUCTIVE idleNow=12s activeRatio=34% samples=1800
```

🛑 Arrêt :

```text
[time] STOP
```

---

## 🛠️ Installation & Build

### ✅ Prérequis

* 🟦 Go 1.20+
* 🪟 Windows

---

### 📥 Cloner le projet

```bash
git clone https://github.com/yourusername/asworm.git
cd asworm
```

---

### 🖥️ Compilation (version console)

```bash
go build -o asworm.exe .
```

---

### 🕶️ Compilation (mode background, sans console)

```bash
go build -ldflags="-H=windowsgui" -o asworm.exe .
```

---

## ▶️ Utilisation

Lancer l’exécutable :

```bash
asworm.exe
```

Arrêt :

* `Ctrl+C` en mode console ⌨️
* Ou via Task Manager en mode GUI 🧩

---

## ⚙️ Configuration

Tous les paramètres sont dans la structure `Config` dans `main()` :

| Champ 🔧                  | Description 📌                       |
| ------------------------- | ------------------------------------ |
| `SampleEvery`             | Intervalle d’échantillonnage (1s) ⏱️ |
| `WindowSize`              | Fenêtre glissante (30m) 🕐           |
| `ActiveIfIdleLessThan`    | Seuil activité (30s) ⏳               |
| `HighProductiveRatio`     | Seuil productivité haute (0.60) 💪   |
| `SimpleProductiveRatio`   | Seuil activité moyenne (0.30) 🙂     |
| `ContinuousIdleThreshold` | Idle long → IDLE (30m) 😴            |
| `PrintStatusEvery`        | Fréquence logs statut (30s) 📌       |
| `PrintMouseMoveEvery`     | Limite logs souris (0 = tout) 🖱️    |
| `LogDir`                  | Répertoire des logs 📂               |
| `FlushEvery`              | Sync disque (5s) 💾                  |

---

## ⚠️ Disclaimer

ASWORM peut ressembler à un outil de monitoring car il suit l’inactivité et la souris.
Il est destiné uniquement à :

* la sensibilisation à l’activité
* l’analyse locale de productivité
* l’observation opérationnelle

Veuillez respecter :

* 🏢 les politiques internes
* ⚖️ les lois locales

---

## 📜 Licence

Ajoutez votre licence :

* MIT 🆓
* Apache-2.0 📄
* Proprietary 🔒

---

🐛 **ASWORM — Simple Activity Awareness for Windows Workstations**

```
```
