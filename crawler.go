package main

import (
    "fmt"
    "net/http"
    "io/ioutil"
    "regexp"
    "container/list"
    "strconv"
    "os"
    "flag"
    "strings"
)

type LootTable struct {
    itemid int
    chance float64
    LootTable list.List
}

func extractList(str string) (*list.List, error) {
    re, err := regexp.Compile(`data: (.*?)\}$`);
    if err != nil {
        return nil, err;
    }

    res := re.FindStringSubmatch(str);
    if res == nil {
        return nil, nil;
    }


    //fmt.Printf("Array: %s\n", res[1]);
    re2, err := regexp.Compile(`\{("armor":.*?)\},`);

    res2 := re2.FindAllStringSubmatch(res[1], -1);

    //re3, err := regexp.Compile(`"id":(.*?),.*"count":(.*?),"outof":(.*?)"`);
    re3, err := regexp.Compile(`"id":([0-9]+).*"count":([0-9]+),"outof":([0-9]+)`);
    if err != nil || len(res2) < 1 {
        return nil, nil;
    }

    var l list.List;
    for i := 0; i < len(res2); i++ {
        //fmt.Printf("res[%d][1]: %q\n", i, res2[i]);

        res3 := re3.FindStringSubmatch(res2[i][1]);
        if len(res3) < 3 {
            continue
        }

        itemid, _ := strconv.Atoi(res3[1]);
        count, _ := strconv.ParseFloat(res3[2], 64);
        totalCount, _ :=  strconv.ParseFloat(res3[3], 64);

        chance := (count / totalCount * 100);

        elem := new(LootTable)
        elem.itemid = itemid
        elem.chance = chance
        l.PushBack(elem)

        //fmt.Printf("index: %d, data: %q\n", i, res3);
        fmt.Printf("Item: %d, chance: %f\n", elem.itemid, elem.chance);
    }

    return &l, nil;
}

func getLootTable(body string) (*list.List, error) {

    re, err := regexp.Compile(`new Listview\((.*?)\);`);
    if err != nil {
        return nil, err;
    }

    res := re.FindAllStringSubmatch(body, -1);
    for i := 0; i < len(res); i++ {

        match, err := regexp.MatchString("{template: 'item'", res[i][0]);
        if err != nil {
            return nil, err;
        }
        if match {
            list, err := extractList(res[i][1]);
            return list, err;
        }
    }

    return nil, nil;
}

func getNpcName(body string) (*string) {

    re, err := regexp.Compile(`<title>(.*) -.*-.*</title>`)
    if err != nil {
        return nil
    }

    res := re.FindStringSubmatch(body)
    if len(res) > 1 {
        return &res[1]
    }
    return nil
}


func parsePage(target string) (*list.List, *string, error) {

    fmt.Printf("Opening %s\n", target);

    resp, err := http.Get(target);

    if err != nil {

        fmt.Printf("Error: %s\n", err);
        return nil, nil, err;
    } else {

        defer resp.Body.Close();
        body, err := ioutil.ReadAll(resp.Body);

        if err != nil {

            fmt.Printf("Error: %s\n", err);
            return nil, nil, err;
        } else {

            l, err := getLootTable(string(body));
            if err != nil {
                fmt.Printf("Error: %s\n", err);
                return nil, nil, err;
            }
            name := getNpcName(string(body))
            return l, name, err;
        }
    }

    return nil, nil, nil;
}

func dumpLootFor(entry int, boss int) {

    target := "http://wowhead.com/npc=" + strconv.Itoa(entry);

    l, name, err := parsePage(target);

    if err != nil {
        return;
    }

    if l == nil {
        fmt.Printf("Empty loot for %d\n", entry);
        return;
    }

    if name == nil {
        fmt.Printf("Empty name for %d\n", entry)
        return
    }

    writeLoot(entry, l, name, boss);
}

func writeLoot(entry int, l *list.List, name *string, boss int) {

    file, err := os.OpenFile("loot.sql", os.O_RDWR | os.O_APPEND, 0660)
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }

    fmt.Fprintf(file, "-- %s (%d)\n", *name, entry)
    fmt.Fprintf(file, "DELETE FROM `creature_loot_template` WHERE `entry` = %d;\n", entry);
    fmt.Fprintf(file, "INSERT INTO `creature_loot_template` (`entry`, `item`, `ChanceOrQuestChance`, `lootmode`, `groupid`, `mincountOrRef`, `maxcount`) VALUES\n");

    if boss == 1 {
        fmt.Fprintf(file, "('%d','%d','100','1','0','-%d','1');\n\n", entry, entry, entry);

        fmt.Fprintf(file, "DELETE FROM `reference_loot_template` WHERE `entry` = %d;\n", entry);
        fmt.Fprintf(file, "INSERT INTO `refrence_loot_template` (`entry`, `item`, `ChanceOrQuestChance`, `lootmode`, `groupid`, `mincountOrRef`, `maxcount`) VALUES\n");
    }

    for e := l.Front(); e != nil; e = e.Next() {
        loot := e.Value.(*LootTable);
        sep := ',';
        if e.Next() == nil {
            sep = ';';
        }
        fmt.Fprintf(file, "('%d','%d','%f','1','1','1','1')%c\n", entry, loot.itemid, loot.chance, sep);
    }

    fmt.Fprintf(file, "\n");

    file.Close();
}

func truncFile() error {

    file, err := os.Create("loot.sql")
    if err != nil {
        return err;
    }

    file.Close();
    return nil
}

func Usage() {
    fmt.Printf("Usage: ./GoCrawler [npc_entry]\n")
}

func main() {

    flag.Parse()
    argv := flag.Args();

    if len(argv) == 0 {
        Usage()
        return
    }


    err := truncFile();

    if err != nil {
        fmt.Printf("Error: %s\n", err);
        return;
    }

    for i := 0; i < len(argv); i++ {

        param := strings.Split(argv[i], ":")
        boss := 0
        if len(param) == 2 && param[1] == "1" {
            boss = 1
        }
        npc_entry, err := strconv.Atoi(param[0])
        if err != nil {
            fmt.Printf("Error: %s\n", err)
            continue
        }
        dumpLootFor(npc_entry, boss)
    }

}
