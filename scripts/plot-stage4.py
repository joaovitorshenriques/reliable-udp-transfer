"""Validate a stage 4 campaign and plot sender means with Student t 95% CI."""
import argparse
import csv
import importlib.util
import json
import os
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
# Optional project-local installation; a normal matplotlib installation also works.
local = ROOT / ".tools" / "python-packages"
if local.is_dir():
    sys.path.insert(0, str(local))
os.environ.setdefault("MPLCONFIGDIR", str(ROOT / ".cache" / "matplotlib"))
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt


def plot(campaign, output):
    spec = importlib.util.spec_from_file_location("analysis", ROOT / "scripts" / "analyze-stage4.py")
    analysis = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(analysis)
    analysis.analyze(campaign, output)
    with (output / "summary.csv").open(encoding="utf-8", newline="") as handle:
        rows = list(csv.DictReader(handle))
    manifest = json.loads((campaign / "manifest.json").read_text(encoding="utf-8"))
    groups = {g["Name"]: g for g in manifest["Groups"]}
    names = sorted(groups)
    windows = ", ".join(str(w) for w in sorted({g["Window"] for g in groups.values()}))
    labels = [f"{n}\n{groups[n]['RTT']/1e6:g} ms; {100*groups[n]['Loss']:g}%" for n in names]
    plt.rcParams.update({"font.size": 11, "axes.spines.top": False, "axes.spines.right": False})
    charts = [
        ("goodput", "goodput_mbps", "Goodput do emissor (Mbit/s)", True),
        ("tempo", "seconds", "Tempo do emissor (s)", True),
        ("rtt", "mean_data_rtt_ms", "RTT médio DATA/ACK (ms)", False),
        ("retransmissoes", "data_retransmissions", "Retransmissões DATA por transferência", False),
    ]
    for filename, field, ylabel, log in charts:
        fig, ax = plt.subplots(figsize=(8, 5.3), layout="constrained")
        for operation, offset, color, marker in [("upload", -.10, "#2166ac", "o"), ("download", .10, "#b35806", "s")]:
            selected = {r["scenario"]: r for r in rows if r["operation"] == operation}
            ax.errorbar([i+offset for i in range(len(names))],
                        [float(selected[n][field+"_mean"]) for n in names],
                        yerr=[float(selected[n][field+"_ci95_halfwidth"]) for n in names],
                        fmt=marker, color=color, capsize=4, label=operation.capitalize())
        if field == "mean_data_rtt_ms":
            ax.scatter(range(len(names)), [groups[n]["RTT"]/1e6 for n in names],
                       marker="_", s=220, color="#333333", label="RTT adicionado")
        if log:
            ax.set_yscale("log")
            ylabel += " — escala log"
        else:
            ax.set_ylim(bottom=-.025*ax.get_ylim()[1])
        ax.set_xticks(range(len(names)), labels)
        ax.set_xlabel("Cenário: RTT adicionado; perda por sentido")
        ax.set_ylabel(ylabel)
        ax.set_title(f"{manifest['MiB']} MiB · janela {windows} · média e IC95% (n={manifest['Repetitions']})")
        ax.legend(frameon=False)
        ax.grid(False)
        for extension in ("png", "pdf"):
            fig.savefig(output / f"{filename}.{extension}", dpi=200)
        plt.close(fig)
    report = output / "analysis.md"
    text = report.read_text(encoding="utf-8").replace("variação de janela, gráficos e capturas Wireshark", "variação de janela e capturas Wireshark")
    text += "\n## Gráficos\n\nBarras: IC95% t de Student entre cinco repetições; métricas do emissor nas duas operações. RTT usa apenas amostras sem retransmissão (Karn). Os gráficos de goodput e tempo usam escala logarítmica. Arquivos PDF de mesmo nome também estão disponíveis.\n"
    for filename, _, ylabel, _ in charts:
        text += f"\n![{ylabel}]({filename}.png)\n"
    report.write_text(text, encoding="utf-8")
    print("Plots:", output)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("campaign", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    plot(args.campaign, args.out)
