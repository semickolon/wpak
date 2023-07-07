{
  description = "wpak";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-23.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let 
        pkgs = import nixpkgs { inherit system; };
        wpak = pkgs.buildGoModule rec {
          name = "wpak";
          src = ./.;
          vendorHash = null;
        };
      in
      {
        packages.default = wpak;
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            go-outline
            delve
            go-tools
          ];
        };
      }
    );
}
