{
  description = "Unix-style Matrix bots";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "messages";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-1cBPDVfJNKApZwcUuIzgEc3KbgX09dNDeU1EHLFo61c=";
          subPackages = [ "cmd/messages" ];
          tags = [ "goolm" ];

          meta = with pkgs.lib; {
            description = "Unix-style Matrix bot";
            homepage = "https://github.com/arjungandhi/messages";
            license = licenses.mit;
            mainProgram = "messages";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
          ];
        };
      }
    );
}
