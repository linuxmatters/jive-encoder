{
  description = "Podcast encoder (MP3, AAC, Opus) for linuxmatters.sh";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:

    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            actionlint
            cosign
            curl
            ffmpeg
            gnugrep
            gcc
            go_1_26
            gocyclo
            golangci-lint
            ineffassign
            just
            lame
            mediainfo
          ];
        };
      }
    );
}
