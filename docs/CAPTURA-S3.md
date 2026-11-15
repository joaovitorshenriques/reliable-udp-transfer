# Captura S3 — 13/11/2026

Execução: `go run ./cmd/experiment -stage4 -scenario 3 -repeats 1 -mib 1 -window 64`.
Resultados: `results/etapa4-2075735803`. Captura: `results/captures/s3-w64.pcapng`.

Wireshark/TShark 4.6.9, Npcap 1.88, interface NPF_Loopback, filtro `udp and udp[8:4] = 0x52433255`.

O capinfos confirmou 3.773 pacotes em 16,5144266 s. O dumpcap informou zero descartes na captura. O emulador encaminhou 1.882 datagramas cliente→servidor e 1.891 servidor→cliente: a soma coincide com a contagem capturada. Essa igualdade é uma conferência de contadores, não uma comparação individual dos pacotes.

| Métrica | Upload | Download |
|---|---:|---:|
| SHA-256 no destino | OK | OK |
| Retransmissões DATA do emissor | 93 | 152 |
| RTT médio DATA/ACK do emissor (ms) | 100,876 | 100,975 |
| Goodput do emissor (Mbit/s) | 1,883 | 1,678 |

O goodput de download mostrado no terminal (1,713 Mbit/s) pertence ao receptor; a tabela usa o emissor para ambas as operações.

Perdas sorteadas: 102/1.984 (5,141%) cliente→servidor e 105/1.996 (5,261%) servidor→cliente. Zero descartes por fila, erros de envio e pendentes/cancelados. Os descartes sorteados ocorrem antes do envio ao socket e não aparecem no PCAP. Zero descarte do capturador não significa zero perda no experimento.

O atraso é aplicado antes de cada envio ao socket. Portanto, os timestamps de loopback não medem diretamente todo o RTT virtual observado pela aplicação. O RTT da tabela vem das métricas do protocolo, com amostras sem retransmissão (Karn).

Esta execução de 1 MiB é evidência de funcionamento/captura, não substitui a campanha de cinco repetições de 80 MiB. A inspeção automática validou CRC/tamanho/versão dos 3.773 pacotes e identificou 143 DATA repetidos. Exemplos com frames e sequências estão em [capture-s3/capture.md](capture-s3/capture.md); packets.csv inclui DATA e ACK para correlação.

SHA-256 do PCAP: `13f28141dc105d43af4c0b656d046b799bef51f39a93cef199febcf8668b3a93`.
