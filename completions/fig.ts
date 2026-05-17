// // fig completion spec for unicli
// // Compatible with Fig and Warp terminals for inline ghost-text suggestions
// //
// // Docs: https://fig.io/docs/guides/autocomplete-spec
// // To update: edit this file and open a PR — Fig picks it up automatically

// const globalOptions: Fig.Option[] = [
//   {
//     name: ["--verbose", "-v"],
//     description: "Show detailed output",
//   },
//   {
//     name: ["--quiet", "-q"],
//     description: "Suppress output except errors",
//   },
//   {
//     name: "--dry-run",
//     description: "Show what would happen without executing",
//   },
//   {
//     name: ["--yes", "-y"],
//     description: "Skip confirmation prompts",
//   },
//   {
//     name: "--version",
//     description: "Print version and exit",
//   },
//   {
//     name: ["--help", "-h"],
//     description: "Show help",
//   },
// ];

// const downloadFormats: Fig.Suggestion[] = [
//   { name: "mp4", description: "MPEG-4 video" },
//   { name: "mp3", description: "MP3 audio" },
//   { name: "webm", description: "WebM video" },
//   { name: "mkv", description: "Matroska video" },
//   { name: "m4a", description: "MPEG-4 audio" },
//   { name: "flac", description: "FLAC lossless audio" },
//   { name: "wav", description: "WAV audio" },
//   { name: "ogg", description: "Ogg Vorbis audio" },
// ];

// const downloadQualities: Fig.Suggestion[] = [
//   { name: "best", description: "Best available quality" },
//   { name: "1080p", description: "Full HD" },
//   { name: "720p", description: "HD" },
//   { name: "480p", description: "SD" },
//   { name: "360p", description: "Low" },
//   { name: "240p", description: "Very low" },
// ];

// const completionSpec: Fig.Spec = {
//   name: "unicli",
//   description: "A fast, modular CLI for downloading and transforming media",
//   options: globalOptions,
//   subcommands: [
//     // ── setup ──────────────────────────────────────────────────────────────
//     {
//       name: "setup",
//       description: "Install required dependencies and configure unicli",
//       options: [
//         {
//           name: "--update",
//           description: "Re-download latest versions of all engines",
//         },
//         ...globalOptions,
//       ],
//     },

//     // ── download ───────────────────────────────────────────────────────────
//     {
//       name: "download",
//       description:
//         "Download from any URL — YouTube, Instagram, direct files and more",
//       args: {
//         name: "url",
//         description: "URL to download from",
//       },
//       options: [
//         {
//           name: ["--output", "-o"],
//           description: "Output directory (default: current directory)",
//           args: {
//             name: "directory",
//             description: "Path to output directory",
//             template: "folders",
//           },
//         },
//         {
//           name: ["--format", "-f"],
//           description: "Force output format",
//           args: {
//             name: "format",
//             description: "Output format",
//             suggestions: downloadFormats,
//           },
//         },
//         {
//           name: "--quality",
//           description: "Video quality",
//           args: {
//             name: "quality",
//             description: "Quality level",
//             suggestions: downloadQualities,
//           },
//         },
//         {
//           name: "--audio-only",
//           description: "Extract audio only",
//         },
//         {
//           name: "--no-metadata",
//           description: "Skip embedding metadata",
//         },
//         ...globalOptions,
//       ],
//     },

//     // ── completion ─────────────────────────────────────────────────────────
//     {
//       name: "completion",
//       description: "Manage shell completion scripts",
//       subcommands: [
//         {
//           name: "install",
//           description: "Install completion script into your shell config",
//           options: [
//             {
//               name: "--shell",
//               description: "Shell to install for",
//               args: {
//                 name: "shell",
//                 suggestions: [
//                   { name: "bash", description: "Bash shell" },
//                   { name: "zsh", description: "Zsh shell" },
//                   { name: "fish", description: "Fish shell" },
//                 ],
//               },
//             },
//             ...globalOptions,
//           ],
//         },
//         {
//           name: "bash",
//           description: "Print bash completion script to stdout",
//         },
//         {
//           name: "zsh",
//           description: "Print zsh completion script to stdout",
//         },
//         {
//           name: "fish",
//           description: "Print fish completion script to stdout",
//         },
//       ],
//     },

//     // ── alias ──────────────────────────────────────────────────────────────
//     {
//       name: "alias",
//       description: "Manage the unicli invocation alias",
//       subcommands: [
//         {
//           name: "set",
//           description: "Set a custom alias for unicli",
//           args: {
//             name: "name",
//             description: "The alias name to use (e.g. dl)",
//           },
//         },
//         {
//           name: "get",
//           description: "Show the current alias",
//         },
//         {
//           name: "reset",
//           description: "Remove alias and return to unicli",
//         },
//       ],
//     },
//   ],
// };

// export default completionSpec;

// fig completion spec for unicli
// Compatible with Fig and Warp terminals for inline ghost-text suggestions
//
// Docs: https://fig.io/docs/guides/autocomplete-spec
// To update: edit this file and open a PR — Fig picks it up automatically

// ── Minimal inline types (no external dependency needed) ──────────────────

interface Option {
  name: string | string[];
  description?: string;
  args?: Arg;
}

interface Arg {
  name: string;
  description?: string;
  template?: "folders" | "filepaths";
  suggestions?: Suggestion[];
}

interface Suggestion {
  name: string;
  description?: string;
}

interface Subcommand {
  name: string;
  description?: string;
  args?: Arg;
  options?: Option[];
  subcommands?: Subcommand[];
}

interface Spec {
  name: string;
  description?: string;
  options?: Option[];
  subcommands?: Subcommand[];
}

// ── Shared ─────────────────────────────────────────────────────────────────

const globalOptions: Option[] = [
  { name: ["--verbose", "-v"], description: "Show detailed output" },
  { name: ["--quiet", "-q"], description: "Suppress output except errors" },
  {
    name: "--dry-run",
    description: "Show what would happen without executing",
  },
  { name: ["--yes", "-y"], description: "Skip confirmation prompts" },
  { name: "--version", description: "Print version and exit" },
  { name: ["--help", "-h"], description: "Show help" },
];

const downloadFormats: Suggestion[] = [
  { name: "mp4", description: "MPEG-4 video" },
  { name: "mp3", description: "MP3 audio" },
  { name: "webm", description: "WebM video" },
  { name: "mkv", description: "Matroska video" },
  { name: "m4a", description: "MPEG-4 audio" },
  { name: "flac", description: "FLAC lossless audio" },
  { name: "wav", description: "WAV audio" },
  { name: "ogg", description: "Ogg Vorbis audio" },
];

const downloadQualities: Suggestion[] = [
  { name: "best", description: "Best available quality" },
  { name: "1080p", description: "Full HD" },
  { name: "720p", description: "HD" },
  { name: "480p", description: "SD" },
  { name: "360p", description: "Low" },
  { name: "240p", description: "Very low" },
];

// ── Spec ───────────────────────────────────────────────────────────────────

const completionSpec: Spec = {
  name: "unicli",
  description: "A fast, modular CLI for downloading and transforming media",
  options: globalOptions,
  subcommands: [
    // ── setup ──────────────────────────────────────────────────────────────
    {
      name: "setup",
      description: "Install required dependencies and configure unicli",
      options: [
        {
          name: "--update",
          description: "Re-download latest versions of all engines",
        },
        ...globalOptions,
      ],
    },

    // ── download ───────────────────────────────────────────────────────────
    {
      name: "download",
      description:
        "Download from any URL — YouTube, Instagram, direct files and more",
      args: {
        name: "url",
        description: "URL to download from",
      },
      options: [
        {
          name: ["--output", "-o"],
          description: "Output directory (default: current directory)",
          args: { name: "directory", template: "folders" },
        },
        {
          name: ["--format", "-f"],
          description: "Force output format",
          args: { name: "format", suggestions: downloadFormats },
        },
        {
          name: "--quality",
          description: "Video quality",
          args: { name: "quality", suggestions: downloadQualities },
        },
        { name: "--audio-only", description: "Extract audio only" },
        { name: "--no-metadata", description: "Skip embedding metadata" },
        ...globalOptions,
      ],
    },

    // ── completion ─────────────────────────────────────────────────────────
    {
      name: "completion",
      description: "Manage shell completion scripts",
      subcommands: [
        {
          name: "install",
          description: "Install completion script into your shell config",
          options: [
            {
              name: "--shell",
              description: "Shell to install for",
              args: {
                name: "shell",
                suggestions: [
                  { name: "bash", description: "Bash shell" },
                  { name: "zsh", description: "Zsh shell" },
                  { name: "fish", description: "Fish shell" },
                ],
              },
            },
            ...globalOptions,
          ],
        },
        { name: "bash", description: "Print bash completion script to stdout" },
        { name: "zsh", description: "Print zsh completion script to stdout" },
        { name: "fish", description: "Print fish completion script to stdout" },
      ],
    },

    // ── alias ──────────────────────────────────────────────────────────────
    {
      name: "alias",
      description: "Manage the unicli invocation alias",
      subcommands: [
        {
          name: "set",
          description: "Set a custom alias for unicli",
          args: {
            name: "name",
            description: "The alias name to use (e.g. dl)",
          },
        },
        { name: "get", description: "Show the current alias" },
        { name: "reset", description: "Remove alias and return to unicli" },
      ],
    },
  ],
};

export default completionSpec;
