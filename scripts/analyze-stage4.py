"""Validate and summarize a completed stage 4 campaign using Python stdlib only.

Usage: python scripts/analyze-stage4.py results/etapa4-ID --out docs/results-etapa4-w64
The t intervals describe variation between runs; they do not remove systematic bias.
"""
import argparse
import csv
import hashlib
import json
import math
import statistics as st
from pathlib import Path


def require(condition, message):
    if not condition:
        raise ValueError(message)


def analyze(source, output):
    manifest = json.loads((source / "manifest.json").read_text(encoding="utf-8-sig"))
    rows = [json.loads(line) for line in (source / "metrics.jsonl").read_text(encoding="utf-8-sig").splitlines() if line.strip()]
    emulation = [json.loads(line) for line in (source / "emulation.jsonl").read_text(encoding="utf-8-sig").splitlines() if line.strip()]
    with (source / "metrics.csv").open(encoding="utf-8-sig", newline="") as f:
        csv_rows = list(csv.DictReader(f))
    groups = {g["Name"]: g for g in manifest["Groups"]}
    repeats = manifest["Repetitions"]
    require(repeats == 5, "This report uses Student t(df=4); exactly five repetitions required")
    require((source / "COMPLETE").is_file(), "Missing COMPLETE marker")
    require(len(rows) == len(groups)*repeats*4, "Unexpected number of transfer records")
    require(len(emulation) == len(groups)*repeats, "Unexpected number of emulator records")
    require(len(csv_rows) == len(rows), "CSV/JSONL row count mismatch")
    identities = set()
    sessions = {}
    for r, c in zip(rows, csv_rows):
        m = r["metrics"]
        identity = (r["group"], r["repetition"], r["operation"], r["endpoint"])
        require(identity not in identities, f"Duplicate record {identity}")
        identities.add(identity)
        require(r["success"] is True, f"Failed transfer {identity}")
        require(r["group"] in groups and 1 <= r["repetition"] <= repeats, "Invalid group/repetition")
        require(r["operation"] in ["upload","download"] and r["endpoint"] in ["client","server"], "Invalid operation/endpoint")
        require(m["bytes"] == manifest["MiB"]*1024*1024 and m["sha256"] == manifest["SHA256"], "Size/hash mismatch")
        require(m["window_packets"] == groups[r["group"]]["Window"], "Window mismatch")
        require(m["seconds"] > 0 and math.isfinite(m["seconds"]), "Invalid duration")
        require(math.isclose(m["goodput_mbps"], m["bytes"]*8/m["seconds"]/1e6), "Invalid goodput")
        expected_role = "sender" if (r["operation"], r["endpoint"]) in [("upload", "client"), ("download", "server")] else "receiver"
        require(m["role"] == expected_role, "Wrong endpoint role")
        require(c["session"] == m["session"] and c["group"] == r["group"] and c["operation"] == r["operation"] and c["endpoint"] == r["endpoint"], "CSV/JSONL identity mismatch")
        require(int(c["repetition"]) == r["repetition"] and int(c["bytes"]) == m["bytes"] and c["sha256"] == m["sha256"], "CSV repetition/size/hash mismatch")
        for field in ["seconds", "goodput_mbps", "data_retransmissions", "mean_data_rtt_ms"]:
            require(math.isclose(float(c[field]), m[field]), f"CSV mismatch: {field}")
        sessions.setdefault(m["session"], []).append(r)
    require(len(sessions) == len(groups)*repeats*2, "Wrong session count")
    for pair in sessions.values():
        require(len(pair) == 2 and {r["metrics"]["role"] for r in pair} == {"sender", "receiver"}, "Unpaired session")
        require(len({(r["group"],r["repetition"],r["operation"]) for r in pair}) == 1, "Mismatched pair")
    emulator_ids = set()
    for e in emulation:
        key = (e["group"],e["repetition"])
        require(key not in emulator_ids, "Duplicate emulator pair")
        emulator_ids.add(key)
        g = groups[e["group"]]
        require(e["window_packets"] == g["Window"] and e["loss_probability_per_direction"] == g["Loss"] and e["added_rtt_ms"] == g["RTT"]/1e6, "Emulator settings mismatch")
        require(e["client_seed"] == g["Seed"]+1000*e["repetition"], "Seed mismatch")
        for direction in ["client_to_server", "server_to_client"]:
            d = e[direction]
            require(d["datagrams_seen"] == sum(d[k] for k in ["random_drops", "queue_drops", "forwarded", "send_errors", "pending_or_canceled"]), "Emulator accounting mismatch")
            require(d["queue_drops"] == 0 and d["send_errors"] == 0, "Uncontrolled emulator drops/errors")
            require(math.isclose(d["observed_random_loss"], d["random_drops"]/d["datagrams_seen"]), "Invalid loss ratio")
            if g["Loss"] == 0:
                require(d["random_drops"] == 0, "Unexpected injected loss")
    summary = []
    for name in sorted(groups):
        for operation in ["upload", "download"]:
            selected = [r["metrics"] for r in rows if r["group"] == name and r["operation"] == operation and r["metrics"]["role"] == "sender"]
            require(len(selected) == 5, "Missing repetitions")
            s = {"scenario":name,"operation":operation,"n":len(selected)}
            for field in ["seconds", "goodput_mbps", "mean_data_rtt_ms", "data_retransmissions"]:
                values = [m[field] for m in selected]
                s[field+"_mean"] = st.mean(values)
                s[field+"_sd"] = st.stdev(values)
                s[field+"_ci95_halfwidth"] = 2.7764451051977987*st.stdev(values)/math.sqrt(5)
            s["sender_rc2_mbps_mean"] = st.mean(m["transport_bytes_sent"]*8/m["seconds"]/1e6 for m in selected)
            s["retransmissions_total"] = sum(m["data_retransmissions"] for m in selected)
            summary.append(s)
    output.mkdir(parents=True,exist_ok=True)
    with (output/"summary.csv").open("w",encoding="utf-8",newline="") as f:
        writer=csv.DictWriter(f,fieldnames=list(summary[0]));writer.writeheader();writer.writerows(summary)
    report = ["# Campanha da etapa 4 — janela 64", "", f"Origem: `{source.name}`; {manifest['MiB']} MiB por transferência; Windows/amd64, {manifest['GoVersion']}.", "", f"Validação: {len(rows)} registros, {len(sessions)} sessões pareadas, {len(emulation)} pares upload/download. Todos com sucesso, tamanho e SHA-256 iguais ao manifesto. CSV e JSONL conferidos; zero descartes por fila cheia e zero erros de envio no emulador.", "", f"SHA-256 dos arquivos transferidos: `{manifest['SHA256']}`.", "", "## Métricas do emissor", "", "Médias de cinco repetições. Goodput: média ± semiamplitude do IC95% t de Student (4 graus de liberdade). O intervalo pressupõe repetições independentes e distribuição aproximadamente normal; não cobre erro sistemático da emulação. RTT é a média das médias de cada execução, com Karn. Os tempos de download abaixo são do servidor emissor, diferentes dos tempos de receptor mostrados no terminal.", "", "| Cenário | Operação | Tempo (s) | Goodput ± IC95% (Mbit/s) | RTT (ms) | Retransmissões (média) | RC2 oferecido (Mbit/s) |", "|---|---|---:|---:|---:|---:|---:|"]
    windows = {g["Window"] for g in groups.values()}
    report[0] = "# Campanha da etapa 4 — janelas " + ", ".join(str(w) for w in sorted(windows))
    report[2] = f"Origem: `{source.name}`; {manifest['MiB']} MiB por transferência; {manifest['OS']}/{manifest['Arch']}, {manifest['GoVersion']}."
    for s in summary:
        report.append(f"| {s['scenario']} | {s['operation']} | {s['seconds_mean']:.3f} | {s['goodput_mbps_mean']:.4f} ± {s['goodput_mbps_ci95_halfwidth']:.4f} | {s['mean_data_rtt_ms_mean']:.3f} | {s['data_retransmissions_mean']:.1f} | {s['sender_rc2_mbps_mean']:.4f} |")
    report += ["", "RC2 oferecido inclui cabeçalho do protocolo e retransmissões; exclui UDP/IP/Ethernet e comandos de serviço. Não é throughput capturado no fio.", "", "## Emulador", "", "Taxa observada ponderada: soma dos descartes sorteados / soma dos datagramas vistos. Inclui dados, ACKs e controle nos dois sentidos durante upload e download, não uma taxa separada por operação.", "", "| Cenário | Cliente → servidor | Servidor → cliente | Pendentes/cancelados (total) |", "|---|---:|---:|---:|"]
    for name in sorted(groups):
        selected=[e for e in emulation if e["group"]==name]
        ratios=[100*sum(e[d]["random_drops"] for e in selected)/sum(e[d]["datagrams_seen"] for e in selected) for d in ["client_to_server","server_to_client"]]
        pending=sum(e[d]["pending_or_canceled"] for e in selected for d in ["client_to_server","server_to_client"])
        report.append(f"| {name} | {ratios[0]:.4f}% | {ratios[1]:.4f}% | {pending} |")
    report += ["", "## Interpretação e limites", "", "S1/S2 isolam o aumento do atraso sem perda sorteada; S2/S3 isolam a adição de perda com o mesmo RTT configurado; S3/S4 aumentam o atraso mantendo a probabilidade de perda. Retransmissões em cenário sem perda sorteada não provam perda no enlace: timeouts prematuros, agendamento e perdas fora do emulador exigem investigação própria.", "", "Esta campanha valida as janelas registradas no manifesto, um arquivo e um ambiente. Atrasos são adicionados ao loopback, não medidos entre máquinas. Não foi feita inferência causal sobre cada retransmissão nem teste de significância entre cenários. Ainda faltam variação de janela, gráficos e capturas Wireshark; mini-CDN é outra etapa.", "", "## Proveniência dos dados", ""]
    for name in ["manifest.json","metrics.jsonl","metrics.csv","emulation.jsonl","COMPLETE"]:
        report.append(f"- `{name}`: `{hashlib.sha256((source/name).read_bytes()).hexdigest()}`")
    (output/"analysis.md").write_text("\n".join(report)+"\n",encoding="utf-8")
    for s in summary:
        print(s["scenario"],s["operation"],"goodput=",round(s["goodput_mbps_mean"],4),"CI95 +/-",round(s["goodput_mbps_ci95_halfwidth"],4),"RTT=",round(s["mean_data_rtt_ms_mean"],3),"retransmissions=",s["retransmissions_total"])
    print("Report:",output/"analysis.md")


if __name__ == "__main__":
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("campaign",type=Path)
    parser.add_argument("--out",type=Path,required=True)
    args=parser.parse_args()
    analyze(args.campaign,args.out)
