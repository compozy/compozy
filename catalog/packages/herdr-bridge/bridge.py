#!/usr/bin/env python3
"""Ponte compozy -> herdr.

Recebe payload de hook do CompozyOS no stdin e reflete o estado do agente
como uma linha de agente no herdr. Uma linha por agent_name (estavel),
nao por sessao (as sessoes de loop sao efemeras e se repetem).

Nunca falha o hook: qualquer erro vira exit 0.
"""
import fcntl
import http.client
import json
import os
import re
import shlex
import socket
import sys
import time
from urllib.parse import quote

HERDR_SOCK = os.path.expanduser("~/.config/herdr/herdr.sock")
COMPOZY_SOCK = os.path.join(os.environ.get("COMPOZY_HOME", os.path.expanduser("~/.compozy")), "daemon.sock")
STATE_DIR = os.path.expanduser("~/.local/state/herdr-bridge")
MAP_PATH = os.path.join(STATE_DIR, "panes.json")
SPOOL_DIR = os.path.join(STATE_DIR, "spool")
LOG_PATH = os.path.join(STATE_DIR, "bridge.log")
SOURCE = "compozy-bridge"
AGENT_ID = "compozy"

# allowlist: so agente de verdade vira linha.
#   user   = sessao que a pessoa criou
#   system = agentes de loop (reviewer, review_fixer, publisher...)
# fora daqui ficam as sessoes internas do daemon: spawned (memory extractor),
# dream (checkpoint/curator) e o que mais o daemon inventar depois.
ALLOW_SESSION_TYPES = {"user", "system"}

# sessao sem evento ha tanto tempo sai do calculo de atividade da linha.
# Sem isso, um turn.end perdido (crash, daemon reiniciado) deixa a linha presa
# em `working` para sempre, porque hook so roda quando ha evento.
STALE_SESSION_SECONDS = 1800
SKIP_INPUT_CLASSES = {"synthetic_reentry"}

# eventos que tiram a sessao da lista de ativas (em vez de marcar um estado)
TERMINAL_EVENTS = {"session.post_stop", "agent.stopped", "agent.crashed"}
# prioridade ao consolidar varias sessoes do mesmo agente numa linha so
STATE_RANK = {"blocked": 3, "working": 2, "idle": 1}

# loop nao e agente: e um run que cria sessoes de agente. Evento de loop nao
# traz agent_name nem session_id — traz loop_run_id, loop_name, generation e
# status. Por isso loop tem linha propria, separada da linha de agente.
LOOP_EVENTS = {"loop.started", "loop.generation.pre", "loop.generation.post",
               "loop.gate.post", "loop.node.terminal", "loop.terminal",
               "coordinator.decision"}
# task_id de loop: loop.<looprun-id>.g<geracao>[.node.<nome>]
LOOP_TASK_ID = re.compile(r"^loop\.(looprun-[a-z0-9]+)\.g(\d+)(?:\.node\.(.+))?$")
# status de loop.terminal que exige o operador (fica na linha ate ser resolvido)
LOOP_BLOCKED_STATUSES = {"blocked"}
LOOP_CLOSED_STATUSES = {"done", "no-op", "failed", "exhausted", "stalled", "canceled"}
LOOP_STATUS_STATE = {"running": "working", "queued": "idle", "watching": "idle",
                     "needs-approval": "blocked", "paused": "blocked", "blocked": "blocked"}
LOOP_WATCH_SECONDS = 5

# session.attention.changed carrega `from`/`to` (atividade da sessao) e
# `class` — o motivo pelo qual ela quer voce. Vocabulario observado em runtime:
# none (nada), finished (terminou, informativo). O binario tambem conhece
# `clarify`, que e a pergunta viva de `compozy session clarify`.
ATTENTION_CLASS_KEY = "class"
ATTENTION_BENIGN = {"none", "finished", ""}

EVENT_STATE = {
    "session.post_create": "idle",
    "turn.start": "working",
    "message.start": "working",
    "turn.end": "idle",
    "permission.request": "blocked",
    "permission.denied": "blocked",
    "permission.resolved": "working",
    "task.needs_attention": "blocked",
    "task.blocked": "blocked",
    "session.post_stop": "idle",
    "agent.stopped": "idle",
    "agent.crashed": "idle",
}


def log(msg):
    try:
        with open(LOG_PATH, "a") as fh:
            fh.write(f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} {msg}\n")
    except Exception:
        pass


def herdr(method, params):
    """Uma chamada JSON-RPC no socket do herdr. None se o herdr nao estiver de pe."""
    if not os.path.exists(HERDR_SOCK):
        return None
    req = {"id": f"{SOURCE}:{time.time_ns()}", "method": method, "params": params}
    try:
        cli = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        cli.settimeout(2.0)
        cli.connect(HERDR_SOCK)
        cli.sendall((json.dumps(req) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = cli.recv(65536)
            if not chunk:
                break
            buf += chunk
        cli.close()
        line = buf.split(b"\n")[0]
        return json.loads(line) if line else None
    except Exception as exc:
        log(f"herdr {method} falhou: {exc}")
        return None


class Locked:
    """Lock de arquivo: hooks async podem disparar em paralelo."""

    def __init__(self, name=".lock"):
        self.name = name

    def __enter__(self):
        os.makedirs(STATE_DIR, exist_ok=True)
        self.fh = open(os.path.join(STATE_DIR, self.name), "w")
        fcntl.flock(self.fh, fcntl.LOCK_EX)
        return self

    def __exit__(self, *_):
        fcntl.flock(self.fh, fcntl.LOCK_UN)
        self.fh.close()


def load_map():
    try:
        with open(MAP_PATH) as fh:
            return json.load(fh)
    except Exception:
        return {}


def save_map(data):
    os.makedirs(STATE_DIR, exist_ok=True)
    tmp = MAP_PATH + ".tmp"
    with open(tmp, "w") as fh:
        json.dump(data, fh, indent=1)
    os.replace(tmp, MAP_PATH)


def close_row(data, key):
    """Fecha so o pane do bridge. Com lock; falhas mantem a entrada para retry."""
    entry = data[key]
    res = herdr("pane.close", {"pane_id": entry["pane_id"]})
    if res and ("result" in res or (res.get("error") or {}).get("code") == "pane_not_found"):
        data.pop(key, None)
        log(f"linha encerrada para {key}: {entry['pane_id']}")
        return True
    log(f"pane.close sem sucesso para {entry['pane_id']}: {res}")
    return False


def pane_alive(pane_id):
    res = herdr("pane.get", {"pane_id": pane_id})
    return bool(res and "result" in res)


def row_key(workspace_id, agent_name):
    """Chave da linha. Dois workspaces rodando o mesmo agente sao linhas
    distintas — antes disputavam a mesma e o tail misturava as duas."""
    return f"{workspace_id or 'no-ws'}/{agent_name}"


def prune(data):
    """Tira do mapa as entradas cujo pane nao existe mais. Devolve os removidos."""
    dead = [k for k, e in data.items() if not pane_alive(e.get("pane_id", ""))]
    for k in dead:
        data.pop(k, None)
    return dead


def drop_stale(sessions, now=None):
    """Filtra a atividade recente; silencio nao comprova fim de sessao."""
    now = now if now is not None else time.time()
    return {
        sid: info for sid, info in sessions.items()
        if info.get("state") == "blocked"  # esperando voce nao envelhece
        or now - float(info.get("ts") or 0) < STALE_SESSION_SECONDS
    }


def tail_command(key):
    """Comando que roda dentro do pane: eventos das sessoes desta linha."""
    here = os.path.dirname(os.path.abspath(__file__))
    reader = os.path.join(here, "tail.py")
    return f"python3 {shlex.quote(reader)} {shlex.quote(key)}\n"


def ensure_row(data, key, agent_name):
    """Entrada do mapa para essa linha, criando a aba no herdr se preciso.
    Precisa ser chamada com o lock ja segurado."""
    entry = data.get(key)
    if entry and pane_alive(entry.get("pane_id", "")):
        return entry

    res = herdr("tab.create", {"label": f"cz:{agent_name}", "focus": False})
    if not res or "result" not in res:
        return None
    pane_id = res["result"]["root_pane"]["pane_id"]
    tab_id = res["result"]["tab"]["tab_id"]
    # o pane vira um tail util em vez de um shell vazio.
    herdr("pane.send_text", {"pane_id": pane_id, "text": tail_command(key)})
    entry = {"pane_id": pane_id, "tab_id": tab_id, "agent": agent_name, "sessions": {}}
    data[key] = entry
    log(f"linha criada para {agent_name}: {pane_id}")
    return entry


def consolidate(sessions):
    """Estado da linha a partir de todas as sessoes vivas do agente.
    Uma sessao que termina nao pode apagar outra que ainda trabalha."""
    if not sessions:
        return "idle", None
    best_id, best = None, None
    for sid, info in sessions.items():
        rank = STATE_RANK.get(info.get("state"), 0)
        if best is None or rank > STATE_RANK.get(best.get("state"), 0):
            best_id, best = sid, info
    return best.get("state", "idle"), (best_id, best)


def read_attention(payload):
    """True se a sessao precisa de voce, False se nao, None se o formato mudou.
    Nunca deduz: uma linha presa em `blocked` por chute e pior que nao mostrar."""
    if ATTENTION_CLASS_KEY not in payload:
        return None
    cls = str(payload.get(ATTENTION_CLASS_KEY) or "").strip().lower()
    if cls in ATTENTION_BENIGN:
        return False
    # classe fora do vocabulario conhecido: trata como atencao (e o lado
    # seguro para quem esta olhando) e registra para afinar o mapeamento
    log(f"attention class desconhecida: {cls!r}")
    return True


def loop_tail_command(loop_run_id, workspace_id=None):
    """Comando do pane de uma linha de loop: a timeline viva do run."""
    args = ["compozy", "loop", "events", loop_run_id, "--follow"]
    if workspace_id:
        args += ["--workspace", workspace_id]
    return shlex.join(args) + "\n"


def loop_key(workspace_id, loop_name):
    return f"loop/{workspace_id or 'no-ws'}/{loop_name}"


def node_label(task_id):
    """`loop.<run>.g1.node.review.0` -> `review.0`."""
    marker = ".node."
    return task_id.split(marker, 1)[1] if marker in (task_id or "") else None


def find_loop_entry(data, loop_run_id):
    """loop.node.terminal nao traz loop_name, so loop_run_id."""
    for key, entry in data.items():
        if entry.get("kind") == "loop" and (loop_run_id in (entry.get("sessions") or {})
                                            or entry.get("run_id") == loop_run_id):
            return key, entry
    return None, None


def ensure_loop_row(data, key, loop_name, loop_run_id):
    """Linha do loop, criando a aba se preciso. Com o lock ja segurado."""
    entry = data.get(key)
    workspace_id = key.split("/", 2)[1]
    if entry and pane_alive(entry.get("pane_id", "")):
        if entry.get("run_id") != loop_run_id:
            # run novo: troca a timeline que o pane segue
            herdr("pane.send_keys", {"pane_id": entry["pane_id"], "keys": ["ctrl+c"]})
            time.sleep(0.3)
            herdr("pane.send_text", {"pane_id": entry["pane_id"], "text": loop_tail_command(loop_run_id, workspace_id)})
            entry["run_id"] = loop_run_id
        return entry
    res = herdr("tab.create", {"label": f"cz:loop:{loop_name}", "focus": False})
    if not res or "result" not in res:
        return None
    pane_id = res["result"]["root_pane"]["pane_id"]
    tab_id = res["result"]["tab"]["tab_id"]
    herdr("pane.send_text", {"pane_id": pane_id, "text": loop_tail_command(loop_run_id, workspace_id)})
    entry = {"kind": "loop", "pane_id": pane_id, "tab_id": tab_id,
             "loop": loop_name, "run_id": loop_run_id, "sessions": {}}
    data[key] = entry
    log(f"linha de loop criada para {loop_name}: {pane_id}")
    return entry


def handle_loop(payload):
    event = payload.get("event") or ""
    run_id = payload.get("loop_run_id")
    gen_from_task = node_from_task = None
    m = LOOP_TASK_ID.match(str(payload.get("task_id") or ""))
    if m:
        run_id = run_id or m.group(1)
        gen_from_task = int(m.group(2))
        node_from_task = m.group(3)
    if not run_id:
        return
    with Locked():
        data = load_map()
        loop_name = payload.get("loop_name")
        terminal = event == "loop.terminal"
        if loop_name:
            key = loop_key(payload.get("workspace_id"), loop_name)
            entry = data.get(key) if terminal else ensure_loop_row(data, key, loop_name, run_id)
        else:
            key, entry = find_loop_entry(data, run_id)
        if not entry:
            return
        runs = entry.setdefault("sessions", {})
        info = runs.get(run_id) or {"name": entry.get("loop"), "state": "working"}
        info["ts"] = time.time()

        if event == "loop.started":
            info["state"] = "working"
            info["status"] = payload.get("status")
        elif event in ("loop.generation.pre", "loop.generation.post"):
            info["state"] = "working"
            gen = payload.get("generation")
            info["gen"] = gen if gen is not None else gen_from_task
        elif event == "coordinator.decision":
            # sync: o daemon aguarda, entao chega mesmo em loop de 200ms
            info["state"] = "working"
            if gen_from_task is not None:
                info["gen"] = gen_from_task
            if node_from_task:
                info["node"] = f"{node_from_task}:{payload.get('decision') or 'running'}"
        elif event == "loop.gate.post":
            info["gate"] = payload.get("status") or payload.get("disposition")
        elif event == "loop.node.terminal":
            label = node_label(payload.get("task_id"))
            if label:
                info["node"] = f"{label}:{payload.get('disposition') or payload.get('run_status') or '?'}"
        elif event == "loop.terminal":
            status = str(payload.get("status") or "").lower()
            info["status"] = status
            if status in LOOP_BLOCKED_STATUSES:
                info["state"] = "blocked"
            elif status in LOOP_CLOSED_STATUSES:
                runs.pop(run_id, None)   # acabou: sai do calculo da linha
                entry["last_status"] = status
                info = None
            else:
                # Status novo/desconhecido nao comprova que o run acabou.
                return
        if info is not None:
            runs[run_id] = info
        if terminal and not runs:
            close_row(data, key)
            save_map(data)
            return
        row_state, active = consolidate(drop_stale(runs))
        data[key] = entry
        save_map(data)
    report_loop_row(entry, row_state, active, event)


def report_loop_row(entry, row_state, active, event):
    seq = time.time_ns()
    loop_name = entry.get("loop")
    active_id, active_info = active if active else (None, {})
    herdr("pane.report_agent", {
        "pane_id": entry["pane_id"], "source": SOURCE, "agent": AGENT_ID,
        "state": row_state, "seq": seq,
        "agent_session_id": active_id,
        "message": f"{event} · {loop_name}",
    })
    herdr("pane.report_metadata", {
        "pane_id": entry["pane_id"], "source": SOURCE, "seq": seq,
        "display_agent": AGENT_ID,
        "title": f"loop {loop_name}",
        "tokens": {
            "cz_loop": loop_name,
            "cz_run": (active_id or "")[-8:] or None,
            "cz_gen": str(active_info["gen"]) if active_info and active_info.get("gen") is not None else None,
            "cz_node": (active_info or {}).get("node"),
            "cz_status": (active_info or {}).get("status") or entry.get("last_status"),
            "cz_live": str(len(entry["sessions"])) if entry.get("sessions") else None,
        },
    })


def query_loop_status(run_id, workspace_id):
    """Le o briefing pela mesma API local do CLI, sem resolver workspace de novo."""
    if not workspace_id or workspace_id == "no-ws":
        return None
    conn = http.client.HTTPConnection("localhost", timeout=5)
    try:
        conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        conn.sock.settimeout(5)
        conn.sock.connect(COMPOZY_SOCK)
        path = f"/api/workspaces/{quote(workspace_id, safe='')}/loop-runs/{quote(run_id, safe='')}/briefing"
        conn.request("GET", path)
        response = conn.getresponse()
        if response.status != 200:
            log(f"reconcile {run_id}: HTTP {response.status}")
            return None
        return str(json.loads(response.read()).get("status") or "").lower() or None
    except Exception as exc:
        log(f"reconcile {run_id}: {exc}")
        return None
    finally:
        conn.close()


def reconcile_loops():
    """Corrige linhas de loop cujo evento terminal foi cancelado pelo daemon.

    O daemon cancela hook async quando o contexto do passo termina, e
    `loop.terminal` dispara justamente quando o contexto do run fecha — e o
    evento mais exposto. Sem isso a linha fica `working` com o loop ja
    encerrado."""
    fixed = []
    # Consulta fora do lock: um daemon lento nao pode parar os hooks.
    with Locked():
        data = load_map()
        targets = []
        for key, entry in data.items():
            if entry.get("kind") != "loop":
                continue
            workspace_id = key.split("/")[1] if key.count("/") >= 2 else None
            runs = list(entry.get("sessions") or {})
            if not runs and entry.get("run_id"):
                runs = [entry["run_id"]]  # v0.3.1 e anteriores guardavam linhas vazias
            targets.extend((key, entry["pane_id"], run_id, workspace_id) for run_id in runs)
    for key, pane_id, run_id, workspace_id in targets:
        status = query_loop_status(run_id, workspace_id)
        if status not in LOOP_CLOSED_STATUSES and status not in LOOP_STATUS_STATE:
            continue
        with Locked():
            data = load_map()
            entry = data.get(key)
            if not entry or entry["pane_id"] != pane_id:
                continue
            runs = entry.setdefault("sessions", {})
            if run_id not in runs and entry.get("run_id") != run_id:
                continue
            if status in LOOP_CLOSED_STATUSES:
                runs.pop(run_id, None)
                entry["last_status"] = status
                fixed.append((key, run_id, status))
                if not runs:
                    close_row(data, key)
                    save_map(data)
                    continue
            else:
                info = runs.get(run_id) or {"name": entry.get("loop")}
                changed = info.get("state") != LOOP_STATUS_STATE[status] or info.get("status") != status
                info.update(state=LOOP_STATUS_STATE[status], status=status, ts=time.time())
                runs[run_id] = info
                if not changed:
                    save_map(data)
                    continue
                fixed.append((key, run_id, status))
            row_state, active = consolidate(runs)
            report_loop_row(entry, row_state, active, "reconcile")
            save_map(data)
    return fixed


def watch_loops():
    """Um drenador fica vivo enquanto houver loops; os demais saem sem esperar.

    O hook.sh ja o destacou do daemon com nohup. Assim, perder loop.terminal
    nao deixa o pane aberto e a proxima chamada de hook nao precisa chegar.
    """
    os.makedirs(STATE_DIR, exist_ok=True)
    with open(os.path.join(STATE_DIR, ".watch-lock"), "w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        while True:
            reconcile_loops()
            with Locked():
                if not any(e.get("kind") == "loop" for e in load_map().values()):
                    # Solta o lock de watcher antes de permitir um novo hook
                    # criar linha: esse drenador podera assumir o monitor.
                    fcntl.flock(lock, fcntl.LOCK_UN)
                    return
            time.sleep(LOOP_WATCH_SECONDS)


def handle(payload):
    event = payload.get("event") or ""
    if event in LOOP_EVENTS:
        return handle_loop(payload)
    state = EVENT_STATE.get(event)

    if event == "session.attention.changed":
        flag = read_attention(payload)
        if flag is None:
            # formato desconhecido: registra pra calibrar e nao mexe na linha
            log(f"payload nao reconhecido em {event}: {json.dumps(payload)[:2000]}")
            return
        state = "blocked" if flag else "idle"

    if not state:
        return
    if payload.get("session_type") not in ALLOW_SESSION_TYPES:
        return
    if payload.get("input_class") in SKIP_INPUT_CLASSES:
        return
    agent_name = payload.get("agent_name")
    if not agent_name:
        return
    session_id = payload.get("session_id") or "?"
    key = row_key(payload.get("workspace_id"), agent_name)

    with Locked():
        data = load_map()
        # Um encerramento pode chegar duas vezes (agent.stopped e
        # session.post_stop). Nunca cria/recria pane para evento terminal.
        terminal = event in TERMINAL_EVENTS
        entry = data.get(key) if terminal else ensure_row(data, key, agent_name)
        if not entry:
            return
        sessions = entry.setdefault("sessions", {})
        if terminal:
            sessions.pop(session_id, None)
            if not sessions:
                close_row(data, key)
                save_map(data)
                return
        else:
            sessions[session_id] = {
                "state": state,
                "name": payload.get("session_name"),
                "type": payload.get("session_type"),
                "turn": payload.get("turn_id"),
                "ts": time.time(),
            }
        recent = drop_stale(sessions)
        row_state, active = consolidate(recent)
        if active and active[1].get("name"):
            entry["last_title"] = active[1]["name"]
        # Mantem sessoes abertas, inclusive ociosas ou sem evento recente,
        # ate seu terminal: encerrar uma irma nao deve fechar o pane delas.
        live = sum(i.get("state") != "idle" for i in recent.values())
        last_title = entry.get("last_title")
        data[key] = entry
        save_map(data)
        pane_id = entry["pane_id"]

    seq = time.time_ns()
    active_id, active_info = active if active else (None, {})
    herdr("pane.report_agent", {
        "pane_id": pane_id, "source": SOURCE, "agent": AGENT_ID,
        "state": row_state, "seq": seq,
        "agent_session_id": active_id,
        "message": f"{event} · {payload.get('session_name') or ''}".strip(" ·"),
    })
    herdr("pane.report_metadata", {
        "pane_id": pane_id, "source": SOURCE, "seq": seq,
        "display_agent": AGENT_ID,
        "title": (active_info.get("name") if active_info else None) or last_title or agent_name,
        "tokens": {
            "cz_agent": agent_name,
            "cz_session": (active_id or "")[:24] or None,
            "cz_type": active_info.get("type") if active_info else None,
            "cz_live": str(live) if live else None,
        },
    })


def drain_spool():
    """Processa tudo que o hook.sh deixou no spool, em ordem de timestamp.

    Quem pega o lock de drenagem processa a fila inteira — inclusive o que
    outros hook.sh deixaram depois. Isso serializa a ordem mesmo com varios
    drenadores disparados ao mesmo tempo. O lock e outro arquivo (nao o
    `.lock` do mapa) para nao travar com o handle()."""
    if not os.path.isdir(SPOOL_DIR):
        return 0
    handled = 0
    with Locked(".drain-lock"):
        while True:
            batch = []
            for name in os.listdir(SPOOL_DIR):
                if not name.endswith(".json"):
                    continue
                path = os.path.join(SPOOL_DIR, name)
                try:
                    with open(path) as fh:
                        payload = json.load(fh)
                except Exception:
                    try:
                        os.unlink(path)   # meio-escrito ou corrompido
                    except Exception:
                        pass
                    continue
                batch.append((str(payload.get("timestamp") or ""), path, payload))
            if not batch:
                return handled
            batch.sort()
            for _, path, payload in batch:
                try:
                    handle(payload)
                except Exception as exc:
                    log(f"erro em {payload.get('event')}: {exc}")
                finally:
                    try:
                        os.unlink(path)
                    except Exception:
                        pass
                handled += 1


def cmd_status():
    with Locked():
        data = load_map()
        dead = prune(data)
        if dead:
            save_map(data)
    for k in dead:
        print(f"  podada (pane morto): {k}")
    for key, run_id, status in reconcile_loops():
        print(f"  reconciliada: {key} run {run_id[-8:]} -> {status}")
    data = load_map()
    print(f"linhas mapeadas: {len(data)}")
    for key, entry in sorted(data.items()):
        live = drop_stale(entry.get("sessions") or {})
        print(f"  {key:34} {entry['pane_id']:8} {entry['tab_id']:8} sessoes={len(live)}")
        for sid, info in live.items():
            print(f"       {sid:26} {info.get('state')}")
    res = herdr("agent.list", {})
    if res and "result" in res:
        print("\nherdr agent list (linhas do compozy):")
        for a in res["result"]["agents"]:
            if a.get("agent") == AGENT_ID:
                print(f"  {a['pane_id']:8} {a.get('agent_status'):8} {a.get('title') or ''}")


def cmd_reset():
    with Locked():
        data = load_map()
        for key, entry in data.items():
            herdr("pane.release_agent", {
                "pane_id": entry["pane_id"], "source": SOURCE,
                "agent": AGENT_ID, "seq": time.time_ns()})
            herdr("tab.close", {"tab_id": entry["tab_id"]})
            print(f"removida: {key} ({entry['tab_id']})")
        save_map({})


def cmd_refresh():
    """Reinicia o tail dos panes existentes com o comando novo."""
    for key, entry in load_map().items():
        pane_id = entry["pane_id"]
        if entry.get("kind") == "loop":
            cmd = loop_tail_command(entry.get("run_id"), key.split("/", 2)[1])
        else:
            cmd = tail_command(key)
        if not pane_alive(pane_id):
            print(f"  {key}: pane morto, pulando")
            continue
        herdr("pane.send_keys", {"pane_id": pane_id, "keys": ["ctrl+c"]})
        time.sleep(0.3)
        herdr("pane.send_text", {"pane_id": pane_id, "text": cmd})
        print(f"  {key}: tail reiniciado em {pane_id}")


def main():
    if len(sys.argv) > 1:
        if sys.argv[1] == "--drain":
            drain_spool()
            return watch_loops()
        if sys.argv[1] == "--watch-loops":
            return watch_loops()
        if sys.argv[1] == "--refresh":
            return cmd_refresh()
        if sys.argv[1] == "--status":
            return cmd_status()
        if sys.argv[1] == "--reset":
            return cmd_reset()
        print("uso: bridge.py [--drain|--watch-loops|--status|--refresh|--reset]   (sem args: payload no stdin)")
        return
    raw = sys.stdin.read()
    if not raw.strip():
        return
    try:
        handle(json.loads(raw))
    except Exception as exc:
        log(f"erro: {exc}")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        pass
    sys.exit(0)
