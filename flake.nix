{
  description = "Neverland - a writing workspace built with Go and MyGo";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
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
            go_1_27
            gotools
            golangci-lint
            gopls
            nixpkgs-fmt
            jq
            ast-grep
          ];

          GOTOOLCHAIN = "go1.27.3+auto";
          CGO_ENABLED = "0";
        };
      }
    );
}

