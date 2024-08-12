package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// Função para verificar se o usuário é root
func checkRoot() bool {
	return os.Geteuid() == 0
}

// Função para verificar se o rsync está instalado
func checkRsync() bool {
	cmd := exec.Command("which", "rsync")
	output, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(output)) != ""
}

// Função para desmontar a ISO
func unmountISO(isoMountPoint string) error {
	cmd := exec.Command("umount", isoMountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "não montado") {
		return fmt.Errorf("erro ao desmontar o ISO %s: %s", isoMountPoint, output)
	}
	return nil
}

// Função para desmontar partições
func unmountPartition(partitionPath string) error {
	cmd := exec.Command("umount", partitionPath)
	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "não montado") {
		return fmt.Errorf("erro ao desmontar a partição %s: %s", partitionPath, output)
	}
	return nil
}

// Função para listar arquivos ISO em um diretório
func listISOFiles(directory string) ([]string, error) {
	var isoFiles []string
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o diretório %s: %v", directory, err)
	}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".iso" {
			isoFiles = append(isoFiles, filepath.Join(directory, file.Name()))
		}
	}
	return isoFiles, nil
}

// Função para listar dispositivos USB com tamanho e nome
func listUSBDevices() ([]string, error) {
	cmd := exec.Command("lsblk", "-o", "NAME,SIZE,TRAN")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("erro ao listar dispositivos USB: %s", output)
	}

	lines := strings.Split(string(output), "\n")
	var devices []string
	for _, line := range lines {
		if strings.Contains(line, "usb") {
			parts := strings.Fields(line)
			if len(parts) > 2 {
				deviceInfo := fmt.Sprintf("/dev/%s (Tamanho: %s)", parts[0], parts[1])
				devices = append(devices, deviceInfo)
			}
		}
	}
	return devices, nil
}

// Função principal para configurar o pendrive
func setupPendrive(pendrive, isoPath string) {
	partitionPath := fmt.Sprintf("/dev/%s1", strings.TrimPrefix(pendrive, "/dev/"))
	mountPoint := "/mnt/pendrive"
	isoMountPoint := "/mnt/iso"

	fmt.Println("Apagando todas as partições existentes...")
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", pendrive), "bs=1G", "count=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao apagar partições: %s\n", output)
		return
	}

	fmt.Println("Criando nova tabela de partições (MBR)...")
	cmd = exec.Command("fdisk", pendrive)
	cmd.Stdin = strings.NewReader(`o
n
p
1


w
`)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao criar tabela de partições: %s\n", output)
		return
	}

	fmt.Println("Formatando Pendriver em FAT32...")
	if err := unmountPartition(partitionPath); err != nil {
		fmt.Printf("Erro ao desmontar partição %s: %s\n", partitionPath, err)
		return
	}

	bar := progressbar.Default(-1, "Formatando partição...")
	cmd = exec.Command("mkfs.vfat", partitionPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao formatar a partição: %s\n", output)
		return
	}
	bar.Finish()

	fmt.Println("Criando pontos de montagem...")
	cmd = exec.Command("mkdir", "-p", mountPoint)
	_, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao criar ponto de montagem do pendrive: %s\n", err)
		return
	}

	fmt.Println("Criando ponto de ISO")
	cmdM := exec.Command("mkdir", "-p", isoMountPoint)
	_, err = cmdM.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao criar ponto de montagem do pendrive: %s\n", err)
		return
	}
	fmt.Println("Caminho da ISO:", cmdM)

	fmt.Println("Montando o pendrive...")
	cmd = exec.Command("mount", partitionPath, mountPoint)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao montar o pendrive: %s\n", output)
		return
	}

	fmt.Println("Desmontando o ISO se estiver montado...")
	if err := unmountISO(isoMountPoint); err != nil {
		fmt.Printf("Erro ao desmontar o ISO: %s\n", err)
	}

	fmt.Println("Montando o ISO...")
	cmd = exec.Command("mount", "-o", "loop", isoPath, isoMountPoint)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao montar o ISO: %s\n", output)
		return
	}

	// Executa chmod na ISO selecionada
	fmt.Println("Alterando permissões da ISO selecionada...")
	cmd = exec.Command("chmod", "a+r", isoPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao alterar permissões da ISO: %s\n", output)
		return
	}

	fmt.Println("Copiando arquivos do ISO para o pendrive com rsync...")
	bar = progressbar.Default(-1, "Copiando arquivos")
	cmd = exec.Command("rsync", "-a", "--no-owner", "--no-group", "--info=progress2", "--block-size=8192", fmt.Sprintf("%s/", isoMountPoint), mountPoint)
	cmd.Stdout = bar
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Erro ao copiar arquivos para o pendrive com rsync: %s\n", err)
		return
	}
	bar.Finish()

	fmt.Println("Desmontando pontos de montagem...")
	cmd = exec.Command("umount", isoMountPoint)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao desmontar o ISO: %s\n", output)
	}

	cmd = exec.Command("umount", mountPoint)
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Erro ao desmontar o pendrive: %s\n", output)
	}

	fmt.Println("Processo concluído.")
}

func main() {
	// sequncia para limpar o terminal
	fmt.Print("\033[H\033[2J")

	if !checkRoot() {
		fmt.Println("Este programa deve ser executado como root. Por favor, execute com 'sudo'.")
		return
	}

	if !checkRsync() {
		fmt.Println("O rsync não está instalado. Instale-o usando o comando: sudo apt-get install rsync")
		return
	}

	fmt.Println("____________________________________________________")
	fmt.Println("Criador de Boot para Windows no linux ")
	fmt.Println("Deixa a iso ou as isos na pasta em que esta o app")
	fmt.Println("____________________________________________________")

	// Diretório onde o aplicativo está localizado
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Erro ao obter diretório atual: %v\n", err)
		return
	}

	isoDirectory := dir

	// Listar dispositivos USB com informações
	devices, err := listUSBDevices()
	if err != nil {
		fmt.Printf("Erro ao listar dispositivos USB: %v\n", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("Nenhum dispositivo USB encontrado.")
		return
	}

	fmt.Println("Dispositivos USB encontrados:")
	for i, device := range devices {
		fmt.Printf("[%d] %s\n", i+1, device)
	}

	var pendriveChoice int
	fmt.Print("Digite o número correspondente ao dispositivo USB desejado: ")
	fmt.Scan(&pendriveChoice)

	if pendriveChoice < 1 || pendriveChoice > len(devices) {
		fmt.Println("Escolha inválida.")
		return
	}

	// Obtemos o nome do dispositivo sem o formato /dev/
	pendrive := strings.Split(devices[pendriveChoice-1], " ")[0]

	isoFiles, err := listISOFiles(isoDirectory)
	if err != nil {
		fmt.Printf("Erro ao listar ISOs: %v\n", err)
		return
	}

	if len(isoFiles) == 0 {
		fmt.Println("Nenhum arquivo ISO encontrado no diretório especificado.")
		return
	}

	fmt.Println("Selecione o arquivo ISO:")
	for i, iso := range isoFiles {
		fmt.Printf("[%d] %s\n", i+1, filepath.Base(iso))
	}

	var isoChoice int
	fmt.Print("Digite o número correspondente ao ISO desejado: ")
	fmt.Scan(&isoChoice)

	if isoChoice < 1 || isoChoice > len(isoFiles) {
		fmt.Println("Escolha inválida.")
		return
	}

	isoPath := isoFiles[isoChoice-1]

	setupPendrive(pendrive, isoPath)
	fmt.Println("____________________________________________________")
	fmt.Println(`Processo finalizado.. \n.............ass: Erasmo Cardoso`)
	fmt.Println("____________________________________________________")
}
