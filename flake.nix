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
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ (final: prev: { go = prev.go_1_26; }) ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          shellHook = import ./nix/hooks.nix { inherit pkgs; };
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
          ] ++ import ./nix/loader.nix { inherit pkgs; };
        };
      }
    );
}
