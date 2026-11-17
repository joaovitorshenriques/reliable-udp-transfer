# Mini-CDN: campanha LAN

15 transferências de 80 MiB, sessões cliente/servidor pareadas e hashes/tamanhos iguais. Desktop cliente e réplica local (cabo); notebook réplica remota (Wi-Fi). Cinco seleções escolheram o desktop.

| Modalidade | Goodput receptor (Mbit/s), média ± IC95% | RTT emissor médio (ms) | Retransmissões totais |
|---|---:|---:|---:|
| desktop | 282.677 ± 80.751 | 2.115 | 0 |
| notebook | 142.992 ± 7.592 | 3.913 | 1408 |
| cdn | 261.087 ± 76.786 | 2.293 | 0 |

IC95% t (4 graus de liberdade) descreve dispersão de cinco observações; não remove viés do ambiente ou da interrupção. A rodada 5 ocorreu após retomada por limite de oito endpoints e espera de 125 s. A campanha não foi contínua. Não inferir superioridade da mini-CDN frente ao acesso direto local: ela escolheu a mesma máquina e houve variabilidade considerável.

RTT é média das médias DATA/ACK do emissor (Karn), diferente das sondagens de seleção. Retransmissões não equivalem a perda de rede medida. A posição local do cliente favorece a réplica desktop. Os resultados não comparam dois caminhos remotos.

Captura informada pelo dumpcap: 2.150.290 pacotes; Ethernet 718.660 e loopback 1.431.630; zero descartes informados. Inspeção dirigida concluída: veja [evidências da captura](capture.md). As 50 sondagens receberam respostas válidas; as cinco escolhas e as 15 sessões de transferência foram correlacionadas.
