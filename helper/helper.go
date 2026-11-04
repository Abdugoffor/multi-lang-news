package helper

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

var validate = func() *validator.Validate {
	v := validator.New()

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})

	return v
}()

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ .env fayl topilmadi yoki yuklanmadi")
	}
}

func ENV(key string) string {
	return os.Getenv(key)
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Validate(v any) map[string]string {
	err := validate.Struct(v)

	if err == nil {
		return nil
	}

	errs := make(map[string]string)

	validationErrors, ok := err.(validator.ValidationErrors)

	if !ok {
		errs["validation"] = "Ma'lumotlarni tekshirishda xatolik yuz berdi"
		return errs
	}

	for _, e := range validationErrors {
		switch e.Tag() {
		case "required":
			errs[e.Field()] = e.Field() + " maydoni majburiy"
		case "email":
			errs[e.Field()] = e.Field() + " elektron pochta formati noto'g'ri"
		case "min":
			errs[e.Field()] = e.Field() + " kamida " + e.Param() + " ta belgidan iborat bo'lishi kerak"
		case "max":
			errs[e.Field()] = e.Field() + " ko'pi bilan " + e.Param() + " ta belgidan iborat bo'lishi kerak"
		default:
			errs[e.Field()] = e.Field() + " qiymati noto'g'ri"
		}
	}

	return errs
}

var slugReplacer = strings.NewReplacer(
	// O'zbek kirill
	"а", "a",
	"б", "b",
	"в", "v",
	"г", "g",
	"д", "d",
	"е", "e",
	"ё", "yo",
	"ж", "j",
	"з", "z",
	"и", "i",
	"й", "y",
	"к", "k",
	"л", "l",
	"м", "m",
	"н", "n",
	"о", "o",
	"п", "p",
	"р", "r",
	"с", "s",
	"т", "t",
	"у", "u",
	"ф", "f",
	"х", "x",
	"ц", "ts",
	"ч", "ch",
	"ш", "sh",
	"щ", "sh",
	"ъ", "",
	"ы", "y",
	"ь", "",
	"э", "e",
	"ю", "yu",
	"я", "ya",

	// O'zbek maxsus kirill
	"қ", "q",
	"ғ", "g",
	"ҳ", "h",
	"ў", "o",

	// Katta harflar
	"А", "a",
	"Б", "b",
	"В", "v",
	"Г", "g",
	"Д", "d",
	"Е", "e",
	"Ё", "yo",
	"Ж", "j",
	"З", "z",
	"И", "i",
	"Й", "y",
	"К", "k",
	"Л", "l",
	"М", "m",
	"Н", "n",
	"О", "o",
	"П", "p",
	"Р", "r",
	"С", "s",
	"Т", "t",
	"У", "u",
	"Ф", "f",
	"Х", "x",
	"Ц", "ts",
	"Ч", "ch",
	"Ш", "sh",
	"Щ", "sh",
	"Ъ", "",
	"Ы", "y",
	"Ь", "",
	"Э", "e",
	"Ю", "yu",
	"Я", "ya",

	// O'zbek maxsus katta kirill
	"Қ", "q",
	"Ғ", "g",
	"Ҳ", "h",
	"Ў", "o",

	// O'zbek lotin apostroflari
	"ʻ", "",
	"ʼ", "",
	"‘", "",
	"’", "",
	"`", "",
	"'", "",

	// Ba'zi maxsus belgilar
	"&", " va ",
	"+", " plus ",
)

var (
	regSpecialChars = regexp.MustCompile(`[^a-z0-9\s-]`)
	regSpacesDashes = regexp.MustCompile(`[\s-]+`)
)

func Slug(data string) string {
	data = strings.TrimSpace(data)
	data = slugReplacer.Replace(data)
	data = strings.ToLower(data)

	data = regSpecialChars.ReplaceAllString(data, "")
	data = regSpacesDashes.ReplaceAllString(data, "-")
	data = strings.Trim(data, "-")

	return data
}
