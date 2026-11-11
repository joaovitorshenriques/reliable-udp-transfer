# RC2: transferência confiável sobre UDP

Projeto acadêmico de Redes de Computadores II. Linguagem: **Go**, somente biblioteca padrão. O protocolo é próprio, inspirado nos conceitos do enunciado; não é uma implementação do QUIC.

## Estado atual

**Etapa 1: núcleo de transferência e CLI de teste.** Implementados: cabeçalho binário, CRC32 por datagrama, Selective Repeat, janela negociada, timeout fixo, limite de tentativas, reordenação, supressão de duplicatas, SHA-256 e confirmação de término com retransmissão. Arquivos são processados incrementalmente, sem carregar o arquivo inteiro na memória. O receptor aceita uma transferência por execução.

Ainda não implementados: aplicação cliente-servidor com senha, listagem, comandos remotos de upload/download, múltiplas sessões, seleção de servidor, cenários experimentais, dashboard e implantação em nuvem. Os comandos `send`/`receive` permitem transferir em qualquer direção, mas **não substituem esses requisitos de aplicação**.

## Requisitos

- Go 1.22 ou superior, disponível no PATH.
- PowerShell para executar os exemplos abaixo no Windows.

O projeto utiliza somente a biblioteca padrão do Go, sem dependências externas. O compilador está disponível no [site oficial do Go](https://go.dev/dl/).

## Compilação

Na raiz do repositório, execute:

```powershell
go version
go build -o bin/rc2.exe ./cmd/rc2
```

**Instalação portátil no Windows:** se o Go estiver em `.tools/go`, substitua `go` por `.\scripts\go.ps1` nos comandos. O script utiliza caches locais; o compilador e os caches não são versionados.

## Teste manual de transferência

Execute os comandos na raiz do repositório. Os dois terminais devem permanecer nessa pasta.

### 1. Criar o arquivo

Crie um arquivo de teste de **80 MiB** (83.886.080 bytes, acima do mínimo de 80 MB):

```powershell
New-Item -ItemType Directory -Force work | Out-Null
$buffer = New-Object byte[] (1MB)
$random = [System.Random]::new(2026)
$stream = [System.IO.File]::Create((Join-Path $PWD 'work\original.bin'))
try {
    for ($i = 0; $i -lt 80; $i++) {
        $random.NextBytes($buffer)
        $stream.Write($buffer, 0, $buffer.Length)
    }
} finally { $stream.Dispose() }
```

### 2. Iniciar o receptor

No primeiro terminal, execute o comando abaixo. O caminho de saída deve ser novo:

```powershell
.\bin\rc2.exe receive -addr 127.0.0.1:9000 -file work/recebido.bin -idle 2m
```

### 3. Enviar o arquivo

Em outro terminal, inicie o envio em até dois minutos após abrir o receptor:

```powershell
.\bin\rc2.exe send -addr 127.0.0.1:9000 -file work/original.bin
```

### 4. Conferir o resultado

Após a transferência, compare os hashes e tamanhos:

```powershell
Get-FileHash work/original.bin, work/recebido.bin -Algorithm SHA256
Get-Item work/original.bin, work/recebido.bin | Select-Object Name, Length
```

**Aprovação:** `success: true` nos dois processos, tamanho correto e hashes iguais. O receptor aguarda cerca de sete segundos após concluir para responder novamente caso a confirmação final se perca. Essa espera não entra no goodput do receptor. Uma saída já existente é recusada, nunca substituída: use outro nome em cada tentativa.

Para comparar janelas, repita com `-window 1`, `-window 16` e `-window 64` no emissor. Isso é uma verificação exploratória local, não os experimentos finais. No receptor, a janela padrão limita o acordo a 64; para usar 256, ajuste ambos os processos.

## Testes automatizados

Execute a suíte de testes e a análise estática:

```powershell
go test ./... -count=1 -v
go vet ./...
```

Para incluir a transferência de 80 MiB:

```powershell
$env:RC2_LARGE_TEST = '1'
go test ./internal/transfer -run TestLargeFile -v -count=1 -timeout 3m
Remove-Item Env:\RC2_LARGE_TEST
```

### Cobertura

- Corrupção em todos os bytes do cabeçalho e payload, truncamentos e bytes excedentes.
- Arquivo vazio, arquivo parcial e várias janelas completas.
- Perda determinística de START, dados, ACKs e DONE, usando sockets UDP reais de loopback.
- Dados duplicados, dados fora de ordem e um datagrama corrompido.
- Esgotamento de tentativas quando não há resposta.
- Preservação de arquivo existente, recusa de hash incorreto e limpeza do temporário.

Os defeitos injetados pelos testes **não representam uma taxa de perda de 5%** e não servem como resultado experimental do relatório.

## Métricas e limitações

O JSON informa bytes úteis, duração, goodput em Mbit/s, envios de dados, retransmissões de dados, repetições de controle, corrupção observada, duplicatas, janela e SHA-256. No emissor, o RTT médio usa somente ACKs de pacotes nunca retransmitidos (evita ambiguidade de Karn). O RTT do receptor permanece zero, sem amostras; isso não significa latência zero.

`goodput_mbps` mede bytes do arquivo / duração; não inclui overhead UDP/IP. No emissor, a duração inclui negociação e confirmação final, mas não o hash prévio. No receptor, começa ao aceitar START e termina após publicar o arquivo validado. Os dois tempos não são exatamente a mesma grandeza. Ainda serão adicionados throughput no fio e coleta experimental padronizada.

Retransmissões **não são uma medição direta da taxa de perda**: também podem resultar de ACK perdido ou timeout prematuro. A etapa experimental registrará separadamente descarte injetado e estimativas do protocolo.

Esta versão não tem autenticação nem criptografia e deve ser executada em loopback ou rede isolada de teste. CRC32 e SHA-256 não autenticam um remetente; o serviço UDP ainda não deve ser exposto à Internet. Não há controle de congestionamento, RTO adaptativo ou retomada; limite padrão de 1 GiB. O término definitivo depende de um período finito de espera: indisponibilidade prolongada ainda pode causar resultado incerto no emissor após o receptor salvar o arquivo.

A publicação no Windows usa `MoveFile`, que recusa um destino existente. Nos demais sistemas usa hard link e exige suporte do filesystem. O temporário é criado no mesmo diretório do destino. Não há substituição de arquivo preexistente; erro de publicação é reportado e o temporário é removido.

## Documentação

- [Plano de implementação e critérios de avaliação](docs/PLANO.md)
- [Formato do protocolo](docs/PROTOCOLO.md)
- [Registro de validação](docs/VALIDACAO.md)
