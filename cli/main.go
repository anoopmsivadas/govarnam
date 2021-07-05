package main

/**
 * govarnam - An Indian language transliteration library
 * Copyright Subin Siby, 2021
 * Licensed under AGPL-3.0-only
 */

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"gitlab.com/subins2000/govarnam/govarnamgo"
)

var varnam *govarnamgo.VarnamHandle

func logVarnamError() {
	log.Fatal(varnam.GetLastError())
}

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debugging outputs")
	langFlag := flag.String("lang", "", "Language")

	learnFlag := flag.Bool("learn", false, "Learn a word")
	unlearnFlag := flag.Bool("unlearn", false, "Unlearn a word")
	trainFlag := flag.Bool("train", false, "Train a word with a particular pattern. 2 Arguments: Pattern & Word")

	learnFromFileFlag := flag.Bool("learn-from-file", false, "Learn words in a file")

	indicDigitsFlag := flag.Bool("digits", false, "Use indic digits")
	greedy := flag.Bool("greedy", false, "Show only exactly matched suggestions")

	flag.Parse()

	var err error
	varnam, err = govarnamgo.InitFromID(*langFlag)
	if err != nil {
		log.Fatal(err)
	}

	varnam.Debug(*debugFlag)

	config := govarnamgo.Config{IndicDigits: *indicDigitsFlag, DictionarySuggestionsLimit: 10, TokenizerSuggestionsLimit: 10, TokenizerSuggestionsAlways: true}
	varnam.SetConfig(config)

	args := flag.Args()

	if *trainFlag {
		pattern := args[0]
		word := args[1]

		if varnam.Train(pattern, word) {
			fmt.Printf("Trained %s => %s\n", pattern, word)
		} else {
			logVarnamError()
		}
	} else if *learnFlag {
		word := args[0]

		if varnam.Learn(word, 0) {
			fmt.Printf("Learnt %s\n", word)
		} else {
			fmt.Printf("Couldn't learn %s", word)
			logVarnamError()
		}
	} else if *unlearnFlag {
		word := args[0]

		if varnam.Unlearn(word) {
			fmt.Printf("Unlearnt %s\n", word)
		} else {
			fmt.Printf("Couldn't learn %s", word)
			logVarnamError()
		}
	} else if *learnFromFileFlag {
		file, err := os.Open(args[0])
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// C.LearnFromFile(file)
	} else {
		var result govarnamgo.TransliterationResult

		if *greedy {
			// results = C.TransliterateGreedy(args[0])
		} else {
			result = varnam.Transliterate(context.Background(), args[0])
		}

		if len(result.ExactMatch) > 0 {
			fmt.Println("Exact Matches")
			for _, sug := range result.ExactMatch {
				fmt.Println(sug.Word + " " + fmt.Sprint(sug.Weight))
			}
		}
		if len(result.Suggestions) > 0 {
			fmt.Println("Suggestions")
			for _, sug := range result.Suggestions {
				fmt.Println(sug.Word + " " + fmt.Sprint(sug.Weight))
			}
		}
		fmt.Println("Greedy Tokenized")
		for _, sug := range result.GreedyTokenized {
			fmt.Println(sug.Word + " " + fmt.Sprint(sug.Weight))
		}
	}
}