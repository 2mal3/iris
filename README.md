# iris

A streamlined CLI application designed to efficiently rename and organize your pictures and videos by date, making it perfect for locally merging, sorting and backing up your media from multiple sources.

### Features

- consistent renaming and sorting of files based on the time of recording, optimized for viewing in the file browser
- reliable extraction of the recording time from the file metadata or from the filename if data is missing
- automatic handling of duplicate images
- high stability and security through sophisticated error handling

> Although I consider the program to be sufficiently safe, it is still relatively young and has potential bugs. Please consider this before entrusting it with your images.

## Naming and Sorting

Iris renames each file (if a recording date was found) according to the following scheme: `year-month-day_hour-minute-second.extension`. The file is then moved to a subfolder named after the year and quarter in the `year-quarter` format.
However, no normal quatals are used to better accommodate typical vacation periods. Instead, all quatals are moved back by two months. For example, February 2024 will still fall into quarter `2023-4` and December 2024 already into quarter `2025-1`.

## Usage

1. download the appropriate file for your architecture from the releases page
2. create a `config.yaml` file in the same folder as the executable.
3. configure the script to your needs using the sample config
4. run the executable in a terminal

### Example Config

```yaml
input_paths:
  -  "/your/input/path"
output_path: "/your/output/path"
move_files: true
remove_duplicates: false

```

## License

[GPL-3.0](/LICENSE.txt)
