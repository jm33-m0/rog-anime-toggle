package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	root                       = "C:\\Program Files (x86)\\LightingService\\script\\"
	anime_lock                 = root + "anime.lock"
	led_matrix_xml_path        = root + "LedMatrix_LastScript.xml"
	empty_led_matrix_xml_path  = root + "LedMatrix_LastScript.xml.empty"
	backup_led_matrix_xml_path = root + "LedMatrix_LastScript.xml.default"
	empty_led_matrix_xml       = `<root>
    <header>AURA_3.0</header>
    <version>1.0</version>
    <effectProvider>
        <period>-1</period>
        <queue>
            <effect key="anime_GA401IV_0">
                <viewport>GA401IV_0</viewport>
                <trigger>OneTime</trigger>
                <delay>0</delay>
                <duration>-1</duration>
                <layer>0</layer>
            </effect>
        </queue>
    </effectProvider>
    <viewport>
        <group key="GA401IV_0">
            <device>
                <matrix_w>68</matrix_w>
                <matrix_h>28</matrix_h>
                <slash_h>36</slash_h>
                <model>GA401IV</model>
                <type>MATRIX_Laptop</type>
                <direction>0</direction>
                <rgb>0</rgb>
                <index>0</index>
                <location>
                    <x>0</x>
                    <y>0</y>
                </location>
                <usage>
                    <led key="-1" />
                </usage>
            </device>
        </group>
    </viewport>
    <effectList></effectList>
</root>`
)

func runMeElevated() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 //SW_NORMAL

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		fmt.Println(err)
	}
}

func isFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func copyFile(in, out string) (int64, error) {
	i, e := os.Open(in)
	if e != nil {
		return 0, e
	}
	defer i.Close()
	o, e := os.Create(out)
	if e != nil {
		return 0, e
	}
	defer o.Close()
	return o.ReadFrom(i)
}

func IsPrivileged() (result bool) {
	var sid *windows.SID
	token := windows.Token(0) // current user

	// Although this looks scary, it is directly copied from the
	// official windows documentation. The Go API for this is a
	// direct wrap around the official C++ API.
	// See https://docs.microsoft.com/en-us/windows/desktop/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		log.Printf("SID Error: %s", err)
		return false
	}
	result, err = token.IsMember(sid)
	if err != nil {
		log.Printf("Token Membership Error: %s", err)
		return
	}

	return result || token.IsElevated()
}

func main() {
	// check if the script is running as admin
	if !IsPrivileged() {
		runMeElevated()
		os.Exit(0)
	}

	// if anime.lock exists, we assume anime is on
	is_anime_on := isFileExists(anime_lock)

	// first run
	if !isFileExists(empty_led_matrix_xml_path) ||
		!isFileExists(backup_led_matrix_xml_path) {
		log.Print("First run")
		// original led matrix xml
		original_xml, err := ioutil.ReadFile(led_matrix_xml_path)
		if err != nil {
			log.Fatalf("Read original LED matrix XML file: %v", err)
		}
		log.Printf("Original LED matrix XML: %d bytes", len(original_xml))
		// backup original led matrix xml
		err = ioutil.WriteFile(backup_led_matrix_xml_path, original_xml, 0644)
		if err != nil {
			log.Fatalf("Backup LED matrix XML: %v", err)
		}
		log.Printf("Backup LED matrix XML to %s", backup_led_matrix_xml_path)
		// this xml file is used to turn off the led matrix as it has no anime in it
		err = ioutil.WriteFile(empty_led_matrix_xml_path, []byte(empty_led_matrix_xml), 0644)
		if err != nil {
			log.Fatalf("Write empty LED matrix XML: %v", err)
		}
		log.Printf("Write empty LED matrix XML to %s", empty_led_matrix_xml_path)
	}

	// if anime is on, we turn it off
	if is_anime_on {
		_, err := copyFile(empty_led_matrix_xml_path, led_matrix_xml_path)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Turn off anime")
		err = os.Remove(anime_lock)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// else turn it on
		_, err := copyFile(backup_led_matrix_xml_path, led_matrix_xml_path)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Turn on anime")
		_, err = os.Create(anime_lock)
		if err != nil {
			log.Fatal(err)
		}
	}

	// restart lighting service
	err := exec.Command("powershell.exe", "-c", "Restart-Service -Name LightingService").Run()
	if err != nil {
		log.Fatalf("Restarting LightingService: %v", err)
	}
}
