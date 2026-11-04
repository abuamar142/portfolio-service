package game

// DefaultEntries is the shipped word corpus.
//
// Curation rules, applied by hand:
//   - common words a native speaker recognises instantly, no proper nouns
//   - 3–9 letters: shorter is trivial, longer is unreadable when scrambled
//   - no diacritics, no prefixes/suffixes that make anagrams unfair
//   - one category per word, and the category must actually describe it
//
// Anything that fails validation is dropped by NewBank, so a typo here costs a
// word, never a broken game.
func DefaultEntries() map[string][]Entry {
	return map[string][]Entry{
		"id": {
			// benda
			{"buku", "benda"}, {"meja", "benda"}, {"kursi", "benda"}, {"pintu", "benda"},
			{"jendela", "benda"}, {"lampu", "benda"}, {"gelas", "benda"}, {"piring", "benda"},
			{"sendok", "benda"}, {"pisau", "benda"}, {"sepatu", "benda"}, {"baju", "benda"},
			{"topi", "benda"}, {"kunci", "benda"}, {"payung", "benda"}, {"sisir", "benda"},
			{"cangkul", "benda"}, {"cermin", "benda"}, {"tikar", "benda"}, {"ember", "benda"},
			{"kertas", "benda"}, {"pensil", "benda"}, {"gunting", "benda"}, {"sapu", "benda"},

			// kerja
			{"makan", "kerja"}, {"tidur", "kerja"}, {"jalan", "kerja"}, {"baca", "kerja"},
			{"tulis", "kerja"}, {"masak", "kerja"}, {"cuci", "kerja"}, {"beli", "kerja"},
			{"jual", "kerja"}, {"tanya", "kerja"}, {"jawab", "kerja"}, {"dengar", "kerja"},
			{"lihat", "kerja"}, {"pegang", "kerja"}, {"duduk", "kerja"}, {"berdiri", "kerja"},
			{"berlari", "kerja"}, {"melompat", "kerja"}, {"menyanyi", "kerja"}, {"memasak", "kerja"},
			{"membaca", "kerja"}, {"menulis", "kerja"}, {"berjalan", "kerja"}, {"tertawa", "kerja"},

			// sifat
			{"besar", "sifat"}, {"kecil", "sifat"}, {"tinggi", "sifat"}, {"rendah", "sifat"},
			{"cepat", "sifat"}, {"lambat", "sifat"}, {"panas", "sifat"}, {"dingin", "sifat"},
			{"terang", "sifat"}, {"gelap", "sifat"}, {"buruk", "sifat"}, {"mudah", "sifat"},
			{"sulit", "sifat"}, {"ramah", "sifat"}, {"tenang", "sifat"}, {"berani", "sifat"},
			{"rajin", "sifat"}, {"malas", "sifat"}, {"bersih", "sifat"}, {"kotor", "sifat"},

			// hewan
			{"kucing", "hewan"}, {"anjing", "hewan"}, {"burung", "hewan"}, {"ayam", "hewan"},
			{"kambing", "hewan"}, {"gajah", "hewan"}, {"harimau", "hewan"}, {"kelinci", "hewan"},
			{"ular", "hewan"}, {"katak", "hewan"}, {"lebah", "hewan"}, {"semut", "hewan"},
			{"kupu", "hewan"}, {"kuda", "hewan"}, {"bebek", "hewan"}, {"monyet", "hewan"},
			{"ikan", "hewan"}, {"sapi", "hewan"},

			// alam
			{"gunung", "alam"}, {"laut", "alam"}, {"sungai", "alam"}, {"hutan", "alam"},
			{"pantai", "alam"}, {"langit", "alam"}, {"bulan", "alam"}, {"bintang", "alam"},
			{"awan", "alam"}, {"hujan", "alam"}, {"angin", "alam"}, {"pasir", "alam"},
			{"batu", "alam"}, {"api", "alam"}, {"pulau", "alam"}, {"tebing", "alam"},

			// tempat
			{"pasar", "tempat"}, {"sekolah", "tempat"}, {"kantor", "tempat"}, {"rumah", "tempat"},
			{"kota", "tempat"}, {"desa", "tempat"}, {"taman", "tempat"}, {"museum", "tempat"},
			{"dapur", "tempat"}, {"jembatan", "tempat"}, {"bandara", "tempat"}, {"perpustakaan", "tempat"},

			// profesi
			{"guru", "profesi"}, {"dokter", "profesi"}, {"petani", "profesi"}, {"nelayan", "profesi"},
			{"polisi", "profesi"}, {"tentara", "profesi"}, {"pelaut", "profesi"}, {"penulis", "profesi"},
			{"sopir", "profesi"}, {"tukang", "profesi"}, {"pedagang", "profesi"}, {"perawat", "profesi"},

			// makanan
			{"nasi", "makanan"}, {"roti", "makanan"}, {"telur", "makanan"}, {"sayur", "makanan"},
			{"buah", "makanan"}, {"susu", "makanan"}, {"kopi", "makanan"}, {"gula", "makanan"},
			{"garam", "makanan"}, {"sambal", "makanan"}, {"bakso", "makanan"}, {"soto", "makanan"},
			{"tempe", "makanan"}, {"tahu", "makanan"}, {"mie", "makanan"}, {"jagung", "makanan"},

			// tubuh
			{"kepala", "tubuh"}, {"tangan", "tubuh"}, {"kaki", "tubuh"}, {"mata", "tubuh"},
			{"hidung", "tubuh"}, {"mulut", "tubuh"}, {"telinga", "tubuh"}, {"rambut", "tubuh"},
			{"gigi", "tubuh"}, {"lidah", "tubuh"}, {"pipi", "tubuh"}, {"leher", "tubuh"},

			// waktu
			{"pagi", "waktu"}, {"siang", "waktu"}, {"sore", "waktu"}, {"malam", "waktu"},
			{"minggu", "waktu"}, {"tahun", "waktu"}, {"detik", "waktu"}, {"menit", "waktu"},
		},

		"en": {
			// noun
			{"book", "noun"}, {"table", "noun"}, {"chair", "noun"}, {"door", "noun"},
			{"window", "noun"}, {"lamp", "noun"}, {"glass", "noun"}, {"plate", "noun"},
			{"spoon", "noun"}, {"knife", "noun"}, {"shoe", "noun"}, {"shirt", "noun"},
			{"key", "noun"}, {"umbrella", "noun"}, {"broom", "noun"}, {"comb", "noun"},
			{"mirror", "noun"}, {"towel", "noun"}, {"pencil", "noun"}, {"scissors", "noun"},
			{"bottle", "noun"}, {"blanket", "noun"}, {"ladder", "noun"}, {"hammer", "noun"},

			// verb
			{"sleep", "verb"}, {"walk", "verb"}, {"read", "verb"}, {"write", "verb"},
			{"cook", "verb"}, {"wash", "verb"}, {"sell", "verb"}, {"ask", "verb"},
			{"answer", "verb"}, {"hear", "verb"}, {"hold", "verb"}, {"stand", "verb"},
			{"jump", "verb"}, {"sing", "verb"}, {"laugh", "verb"}, {"listen", "verb"},
			{"throw", "verb"}, {"drive", "verb"}, {"build", "verb"}, {"count", "verb"},

			// adjective
			{"big", "adjective"}, {"small", "adjective"}, {"tall", "adjective"}, {"low", "adjective"},
			{"fast", "adjective"}, {"slow", "adjective"}, {"hot", "adjective"}, {"bright", "adjective"},
			{"dark", "adjective"}, {"good", "adjective"}, {"easy", "adjective"}, {"hard", "adjective"},
			{"kind", "adjective"}, {"calm", "adjective"}, {"brave", "adjective"}, {"clean", "adjective"},
			{"dirty", "adjective"}, {"lazy", "adjective"}, {"quiet", "adjective"}, {"proud", "adjective"},

			// animal
			{"cat", "animal"}, {"dog", "animal"}, {"bird", "animal"}, {"fish", "animal"},
			{"chicken", "animal"}, {"cow", "animal"}, {"goat", "animal"}, {"elephant", "animal"},
			{"tiger", "animal"}, {"rabbit", "animal"}, {"snake", "animal"}, {"frog", "animal"},
			{"bee", "animal"}, {"ant", "animal"}, {"butterfly", "animal"}, {"horse", "animal"},
			{"duck", "animal"}, {"monkey", "animal"},

			// nature
			{"mountain", "nature"}, {"river", "nature"}, {"forest", "nature"}, {"beach", "nature"},
			{"sky", "nature"}, {"moon", "nature"}, {"star", "nature"}, {"cloud", "nature"},
			{"rain", "nature"}, {"wind", "nature"}, {"sand", "nature"}, {"stone", "nature"},
			{"fire", "nature"}, {"island", "nature"}, {"valley", "nature"}, {"thunder", "nature"},

			// place
			{"market", "place"}, {"school", "place"}, {"office", "place"}, {"house", "place"},
			{"city", "place"}, {"village", "place"}, {"bridge", "place"}, {"park", "place"},
			{"museum", "place"}, {"kitchen", "place"}, {"airport", "place"}, {"library", "place"},

			// profession
			{"teacher", "profession"}, {"doctor", "profession"}, {"farmer", "profession"},
			{"sailor", "profession"}, {"police", "profession"}, {"writer", "profession"},
			{"driver", "profession"}, {"baker", "profession"}, {"guard", "profession"},
			{"builder", "profession"}, {"painter", "profession"}, {"nurse", "profession"},

			// food
			{"bread", "food"}, {"egg", "food"}, {"vegetable", "food"}, {"fruit", "food"},
			{"milk", "food"}, {"coffee", "food"}, {"sugar", "food"}, {"salt", "food"},
			{"soup", "food"}, {"cake", "food"}, {"rice", "food"}, {"corn", "food"},
			{"cheese", "food"}, {"honey", "food"}, {"onion", "food"}, {"pepper", "food"},

			// body
			{"head", "body"}, {"hand", "body"}, {"foot", "body"}, {"eye", "body"},
			{"nose", "body"}, {"mouth", "body"}, {"ear", "body"}, {"hair", "body"},
			{"tooth", "body"}, {"tongue", "body"}, {"cheek", "body"}, {"neck", "body"},

			// time
			{"morning", "time"}, {"noon", "time"}, {"night", "time"}, {"week", "time"},
			{"month", "time"}, {"year", "time"}, {"hour", "time"}, {"minute", "time"},
			{"second", "time"}, {"today", "time"}, {"tomorrow", "time"}, {"yesterday", "time"},
		},
	}
}
