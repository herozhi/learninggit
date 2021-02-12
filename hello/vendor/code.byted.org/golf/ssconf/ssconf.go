package ssconf

import (
	"bufio"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"

	"code.byted.org/golf/consul"
)

func isSsConfSep(r rune) bool {
	if r == ':' || r == '=' || unicode.IsSpace(r) {
		return true
	}
	return false
}

func parseSsConfKeyValue(line string) (key, value string) {
	// remove comments
	line = strings.Split(strings.Split(line, "#")[0], ";")[0]
	line = strings.TrimSpace(line) // /etc/ss_conf/db_profile.conf  #commentxx, 会多出空格, 导致解析失败
	k_end := -1
	v_begin := -1
	for idx, c := range line {
		is_sep := isSsConfSep(c)
		if k_end == -1 {
			if is_sep {
				k_end = idx
			}
			continue
		}
		if is_sep == false {
			v_begin = idx
			break
		}
	}
	if v_begin == -1 {
		return "", ""
	}
	return line[:k_end], line[v_begin:]
}

func getWholeLine(scanner *bufio.Scanner) (line string, ok bool) {
	ok = false
	var whole_line string
	for scanner.Scan() {
		ok = true
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "\\") {
			// merge line ends with backslash
			line = line[:len(line)-1]
			whole_line += line
			continue
		}
		whole_line += line
		break
	}
	return whole_line, ok
}

var resolvePattern = regexp.MustCompile("\\{\\{\\s*([^\\}]+)\\s*\\}\\}")

func resolve_conf(conf map[string]string) {
	found := true
	for found {
		found = false
		for k, v := range conf { // parse variables
			idx := resolvePattern.FindAllStringIndex(v, 1000)
			lastIdx := 0
			final_v := ""
			for i := range idx {
				st := idx[i][0]
				en := idx[i][1]
				if st >= lastIdx {
					final_v += v[lastIdx:st]
					lastIdx = st
				}
				key := strings.Trim(v[st+2:en-2], " ")
				if real_v, ok := conf[key]; ok {
					final_v += real_v
				} else {
					// resolution not found, use "" instead
					// OTHERWISE the loop never end!
				}
				lastIdx = en
			}
			if lastIdx > 0 {
				if lastIdx < len(v) {
					final_v += v[lastIdx:]
				}
				conf[k] = final_v
				found = true
			}
		}
	}
}

func LoadSsConfFile(filename string) (ret map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	ret = make(map[string]string)
	err = nil

	basename := path.Dir(filename)
	scanner := bufio.NewScanner(file)
	for {
		line, ok := getWholeLine(scanner)
		if ok != true {
			break
		}

		key, value := parseSsConfKeyValue(line)
		if key == "include" {
			include_file := value
			if strings.HasPrefix(include_file, "/") == false {
				include_file = basename + "/" + include_file
			}
			include_conf, err := LoadSsConfFile(include_file)
			if err != nil {
				continue
			}
			// merge sub conf
			for k, v := range include_conf {
				ret[k] = v
			}
		} else if len(key) > 0 {
			ret[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	resolve_conf(ret)
	consul_ret, err := consul.TranslateConf(ret, filename)
	if err == nil {
		ret = consul_ret
	}
	return
}

func LoadSsConfFileOnDemand(filename, consulKey string) (ret map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	ret = make(map[string]string)
	err = nil

	basename := path.Dir(filename)
	scanner := bufio.NewScanner(file)
	for {
		line, ok := getWholeLine(scanner)
		if ok != true {
			break
		}

		key, value := parseSsConfKeyValue(line)
		if key == "include" {
			include_file := value
			if strings.HasPrefix(include_file, "/") == false {
				include_file = basename + "/" + include_file
			}
			include_conf, err := LoadSsConfFile(include_file)
			if err != nil {
				continue
			}
			// merge sub conf
			for k, v := range include_conf {
				ret[k] = v
			}
		} else if len(key) > 0 {
			if key == consulKey {
				ret[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	resolve_conf(ret)
	consul_ret, err := consul.TranslateConf(ret, filename)
	if err == nil {
		ret = consul_ret
	}
	return
}
